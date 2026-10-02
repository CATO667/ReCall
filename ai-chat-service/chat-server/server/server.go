package server

import (
	chat_context "ai-chat-service/chat-server/chat-context"
	"ai-chat-service/chat-server/rerank"
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/chat-server/vector"
	metrics_bus "ai-chat-service/chat-server/metrics-bus"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/log"
	"ai-chat-service/proto"
	"ai-chat-service/services/tokenizer"
	vfclient "ai-chat-service/vecfluxapi/client"
	"context"
	"encoding/json"
	"github.com/golang/protobuf/jsonpb"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"io"
	"time"
)

func llmStreamDeltaText(delta openai.ChatCompletionStreamChoiceDelta) string {
	return delta.Content
}

func llmMessageText(msg openai.ChatCompletionMessage) string {
	return msg.Content
}

func mapStreamChunk(resp openai.ChatCompletionStreamResponse, source proto.ResponseSource) *proto.ChatCompletionStreamResponse {
	out := &proto.ChatCompletionStreamResponse{
		Id:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
	}
	if len(resp.Choices) == 0 {
		return out
	}
	choice := resp.Choices[0]
	deltaText := llmStreamDeltaText(choice.Delta)
	out.Choices = []*proto.ChatCompletionStreamChoice{{
		Index: int32(choice.Index),
		Delta: &proto.ChatCompletionStreamChoiceDelta{
			Content: deltaText,
			Role:    choice.Delta.Role,
		},
		FinishReason: string(choice.FinishReason),
		Source:       source,
	}}
	return out
}

type chatService struct {
	proto.UnimplementedChatServer
	config            *config.Config
	log               log.ILogger
	qaStore           storage.IQAStore
	topicClassifier   *TopicClassifier
	embedder          vector.Embedder
	vectorStore     vector.VectorStore
	rerankService   *rerank.Service
	busMetrics      *metrics_bus.BusMetrics
	vfClient        *vfclient.Client
}

func NewChatService(qaStore storage.IQAStore, topicClassifier *TopicClassifier, embedder vector.Embedder, vectorStore vector.VectorStore, rerankService *rerank.Service, config *config.Config, log log.ILogger, busMetrics *metrics_bus.BusMetrics, vfClient *vfclient.Client) proto.ChatServer {
	return &chatService{
		config:          config,
		log:             log,
		qaStore:         qaStore,
		topicClassifier: topicClassifier,
		embedder:        embedder,
		vectorStore:     vectorStore,
		rerankService:   rerankService,
		busMetrics:      busMetrics,
		vfClient:        vfClient,
	}
}

func (s *chatService) persistLLMResult(app *app, in *proto.ChatCompletionRequest, question string, currMessage openai.ChatCompletionMessage, currTokens int, answer string, completionTokens, promptTokens int, replyID string, cacheQA bool) {
	go func() {
		reqContext := &chat_context.ChatMessage{
			ID:      in.Id,
			PID:     in.Pid,
			Message: currMessage,
			Tokens:  currTokens,
		}
		if err := app.saveContext(reqContext); err != nil {
			s.log.Error(err)
			return
		}
		if replyID == "" {
			replyID = in.Id + "-reply"
		}
		resContext := &chat_context.ChatMessage{
			ID: replyID,
			PID: reqContext.ID,
			Message: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: answer,
			},
			Tokens: completionTokens,
		}
		if err := app.saveContext(resContext); err != nil {
			s.log.Error(err)
			return
		}
	}()
	go func() {
		if !cacheQA {
			return
		}
		total := promptTokens + completionTokens
		normalized := prepareQueryText(question)
		qaHash := storage.QuestionHash(question)

		var emb []float32
		if s.config.Vector.Enabled && s.embedder != nil {
			vec, err := s.embedder.Embed(context.Background(), normalized)
			if err != nil {
				s.log.Error(err)
			} else {
				emb = vec
			}
		}

		topicID, topicName := "", ""
		if s.topicClassifier != nil && len(emb) > 0 {
			id, name, err := s.topicClassifier.AssignTopic(context.Background(), question, qaHash, emb)
			if err != nil {
				s.log.Error(err)
			} else {
				topicID, topicName = id, name
			}
		}

		record := &storage.QACacheRecord{
			Question:         question,
			Answer:           answer,
			Source:           storage.SourceLLM,
			TopicID:          topicID,
			TopicName:        topicName,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      total,
			TokensSaved:      0,
			CreatedAt:        time.Now().Unix(),
		}
		ttl := s.config.Chat.QACacheTTL
		if err := s.qaStore.SetQA(context.Background(), question, record, ttl); err != nil {
			s.log.Error(err)
			return
		}
		if s.config.Vector.Enabled && s.vectorStore != nil && len(emb) > 0 {
			if err := s.vectorStore.Build(context.Background(), "", normalized, emb); err != nil {
				s.log.Error(err)
			}
		}
	}()
}

func (s *chatService) ChatCompletion(ctx context.Context, in *proto.ChatCompletionRequest) (*proto.ChatCompletionResponse, error) {
	contextCache := chat_context.NewKVStoreCache()
	app := s.newApp(in, contextCache)

	ok, msg, err := app.sensitive(in)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	if !ok {
		return app.buildChatCompletionResponse(msg, proto.ResponseSource_SOURCE_SENSITIVE, nil), nil
	}

	if s.vfClient != nil {
		return s.chatCompletionViaVecflux(app, in, ctx)
	}

	ragResult, err := s.resolveRAG(ctx, in.Message)
	if err != nil {
		s.log.Error(err)
	}
	if ragResult.cacheHit {
		usage := &proto.Usage{}
		return app.buildChatCompletionResponse(ragResult.cacheRecord.Answer, proto.ResponseSource_SOURCE_CACHE, usage), nil
	}

	ragRefs := ragResult.ragRefs
	source := responseSourceFromRAG(ragRefs)

	req, tokens, currTokens, currMessage, err := app.buildChatCompletionRequest(in, false, ragRefs)
	if err != nil {
		return nil, err
	}
	resp, err := app.createChatCompletion(ctx, req)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	res := &proto.ChatCompletionResponse{}
	bytes, err := json.Marshal(resp)
	if err != nil {
		s.log.Error(err)
		return nil, err
	}
	if err = jsonpb.UnmarshalString(string(bytes), res); err != nil {
		s.log.Error(err)
		return nil, err
	}
	if len(res.Choices) > 0 {
		res.Choices[0].Source = source
	}
	answer := ""
	if len(resp.Choices) > 0 {
		answer = llmMessageText(resp.Choices[0].Message)
		if len(res.Choices) > 0 && res.Choices[0].Message != nil {
			res.Choices[0].Message.Content = answer
		}
	}
	s.persistLLMResult(app, in, in.Message, currMessage, currTokens, answer, resp.Usage.CompletionTokens, tokens, resp.ID, true)
	return res, nil
}

func (s *chatService) ChatCompletionStream(in *proto.ChatCompletionRequest, stream proto.Chat_ChatCompletionStreamServer) error {
	contextCache := chat_context.NewKVStoreCache()
	app := s.newApp(in, contextCache)

	ok, msg, err := app.sensitive(in)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		s.log.Error(err)
		return err
	}
	if !ok {
		s.busMetrics.SensitiveQuestionsTotalCounter.Inc()
		return app.streamFixedMessage(stream, msg, proto.ResponseSource_SOURCE_SENSITIVE, nil, 0)
	}

	if s.vfClient != nil {
		return s.chatCompletionStreamViaVecflux(app, in, stream)
	}

	ragResult, err := s.resolveRAG(stream.Context(), in.Message)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		s.log.Error(err)
		return err
	}
	if ragResult.cacheHit {
		s.busMetrics.QuestionsTotalCounter.Inc()
		resID := uuid.New().String()
		return app.streamTextChunks(stream, resID, ragResult.cacheRecord.Answer, proto.ResponseSource_SOURCE_CACHE, nil, cacheTokensSaved(ragResult.cacheRecord))
	}

	ragRefs := ragResult.ragRefs
	source := responseSourceFromRAG(ragRefs)

	req, tokens, currTokens, currMessage, err := app.buildChatCompletionRequest(in, true, ragRefs)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		return err
	}
	chatStream, err := app.createChatCompletionStream(stream.Context(), req)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		s.log.Error(err)
		return err
	}
	defer chatStream.Close()

	completionContent := ""
	resultID := ""
	finishReason := ""
	for {
		resp, err := chatStream.Recv()
		if err != nil && err != io.EOF {
			s.log.Error(err)
			return err
		}
		if err == io.EOF {
			break
		}
		if resultID == "" {
			resultID = resp.ID
		}
		if len(resp.Choices) == 0 {
			continue
		}
		choice := resp.Choices[0]
		if fr := string(choice.FinishReason); fr != "" {
			finishReason = fr
		}
		deltaText := llmStreamDeltaText(choice.Delta)
		completionContent += deltaText
		res := mapStreamChunk(resp, source)
		if err = stream.Send(res); err != nil {
			s.log.Error(err)
			return err
		}
	}

	resultMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: completionContent,
	}
	model := s.config.Chat.Model
	if in.ChatParam != nil && in.ChatParam.Model != "" {
		model = in.ChatParam.Model
	}
	resultTokens, err := tokenizer.GetTokens(&resultMessage, model)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		s.log.Error(err)
		return err
	}

	endRes := app.buildChatCompletionStreamResponse(resultID, "", finishReason, source, &proto.Usage{
		PromptTokens:     int32(tokens),
		CompletionTokens: int32(resultTokens),
		TotalTokens:      int32(tokens + resultTokens),
	}, 0)
	if resultID == "" {
		resultID = uuid.New().String()
		endRes.Id = resultID
	}
	if err = stream.Send(endRes); err != nil {
		s.log.Error(err)
		return err
	}

	cacheQA := completionReadyForCache(stream.Context(), finishReason, completionContent)
	if !cacheQA {
		s.log.InfoF("skip QA persist: finishReason=%q completionLen=%d ctxErr=%v", finishReason, len(completionContent), stream.Context().Err())
	}
	s.persistLLMResult(app, in, in.Message, currMessage, currTokens, completionContent, resultTokens, tokens, resultID, cacheQA)
	s.busMetrics.QuestionsTotalCounter.Inc()
	return nil
}
