package server

import (
	chat_context "ai-chat-service/chat-server/chat-context"
	"ai-chat-service/proto"
	"ai-chat-service/services/tokenizer"
	vpb "ai-chat-service/vecfluxapi/proto"
	"context"
	"io"

	"github.com/sashabaranov/go-openai"
)

func buildVGetRequest(app *app, in *proto.ChatCompletionRequest) *vpb.VGetRequest {
	req := &vpb.VGetRequest{
		Question: in.Message,
		Llm: &vpb.LLMOptions{
			Model:             app.openaiConf.Model,
			MaxTokens:         int32(app.openaiConf.MaxTokens),
			Temperature:       app.openaiConf.Temperature,
			TopP:              app.openaiConf.TopP,
			PresencePenalty:   app.openaiConf.PresencePenalty,
			FrequencyPenalty:  app.openaiConf.FrequencyPenalty,
			BotDesc:           app.openaiConf.BotDesc,
			MinResponseTokens: int32(app.openaiConf.MinResponseTokens),
		},
	}
	if in.EnableContext {
		for _, msg := range app.getContext(in.Pid) {
			req.Context = append(req.Context, &vpb.ChatMessage{
				Role:    msg.Message.Role,
				Content: msg.Message.Content,
			})
		}
	}
	return req
}

func mapVFSource(src vpb.ResponseSource) proto.ResponseSource {
	switch src {
	case vpb.ResponseSource_SOURCE_CACHE:
		return proto.ResponseSource_SOURCE_CACHE
	case vpb.ResponseSource_SOURCE_RAG_LLM:
		return proto.ResponseSource_SOURCE_RAG
	case vpb.ResponseSource_SOURCE_PURE_LLM:
		return proto.ResponseSource_SOURCE_LLM
	default:
		return proto.ResponseSource_SOURCE_UNSPECIFIED
	}
}

func (s *chatService) chatCompletionStreamViaVecflux(app *app, in *proto.ChatCompletionRequest, stream proto.Chat_ChatCompletionStreamServer) error {
	vfStream, err := s.vfClient.VGet(stream.Context(), buildVGetRequest(app, in))
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		return err
	}

	requestID := ""
	source := proto.ResponseSource_SOURCE_UNSPECIFIED
	completion := ""
	var usage *proto.Usage
	var tokensSaved int32

	for {
		chunk, err := vfStream.Recv()
		if err != nil && err != io.EOF {
			s.busMetrics.ErrQuestionsTotalCounter.Inc()
			return err
		}
		if err == io.EOF {
			break
		}

		if requestID == "" && chunk.RequestId != "" {
			requestID = chunk.RequestId
		}
		if chunk.Source != vpb.ResponseSource_SOURCE_UNSPECIFIED {
			source = mapVFSource(chunk.Source)
		}
		if chunk.TokensSaved > 0 {
			tokensSaved = chunk.TokensSaved
		}
		if chunk.Usage != nil {
			usage = &proto.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		if chunk.FinishReason == FinishReasonHeartbeat {
			if err := stream.Send(app.buildChatCompletionStreamResponse(requestID, chunk.Delta, FinishReasonHeartbeat, source, nil, 0)); err != nil {
				s.busMetrics.ErrQuestionsTotalCounter.Inc()
				return err
			}
			continue
		}

		if chunk.Done {
			if requestID == "" {
				requestID = chunk.RequestId
			}
			endRes := app.buildChatCompletionStreamResponse(requestID, "", "stop", source, usage, tokensSaved)
			if err := stream.Send(endRes); err != nil {
				s.busMetrics.ErrQuestionsTotalCounter.Inc()
				return err
			}
			break
		}

		if chunk.Delta == "" {
			continue
		}

		completion += chunk.Delta
		if err := stream.Send(app.buildChatCompletionStreamResponse(requestID, chunk.Delta, "", source, nil, 0)); err != nil {
			s.busMetrics.ErrQuestionsTotalCounter.Inc()
			return err
		}
	}

	currMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: in.Message,
	}
	currTokens, err := tokenizer.GetTokens(&currMessage, app.openaiConf.Model)
	if err != nil {
		s.busMetrics.ErrQuestionsTotalCounter.Inc()
		return err
	}
	promptTokens, completionTokens := 0, 0
	if usage != nil {
		promptTokens = int(usage.PromptTokens)
		completionTokens = int(usage.CompletionTokens)
	}
	if source != proto.ResponseSource_SOURCE_CACHE {
		s.persistStreamContext(app, in, currMessage, currTokens, completion, promptTokens, completionTokens, requestID)
	}
	s.busMetrics.QuestionsTotalCounter.Inc()
	return nil
}

func (s *chatService) chatCompletionViaVecflux(app *app, in *proto.ChatCompletionRequest, ctx context.Context) (*proto.ChatCompletionResponse, error) {
	vfStream, err := s.vfClient.VGet(ctx, buildVGetRequest(app, in))
	if err != nil {
		return nil, err
	}

	requestID := ""
	source := proto.ResponseSource_SOURCE_UNSPECIFIED
	answer := ""
	var usage *proto.Usage

	for {
		chunk, err := vfStream.Recv()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if err == io.EOF {
			break
		}
		if requestID == "" && chunk.RequestId != "" {
			requestID = chunk.RequestId
		}
		if chunk.Source != vpb.ResponseSource_SOURCE_UNSPECIFIED {
			source = mapVFSource(chunk.Source)
		}
		if chunk.Usage != nil {
			usage = &proto.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
		if chunk.FinishReason == FinishReasonHeartbeat {
			continue
		}
		if chunk.Delta != "" {
			answer += chunk.Delta
		}
		if chunk.Done {
			break
		}
	}

	currMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: in.Message,
	}
	currTokens, err := tokenizer.GetTokens(&currMessage, app.openaiConf.Model)
	if err != nil {
		return nil, err
	}
	promptTokens, completionTokens := 0, 0
	if usage != nil {
		promptTokens = int(usage.PromptTokens)
		completionTokens = int(usage.CompletionTokens)
	}
	if source != proto.ResponseSource_SOURCE_CACHE {
		s.persistStreamContext(app, in, currMessage, currTokens, answer, promptTokens, completionTokens, requestID)
	}

	res := app.buildChatCompletionResponse(answer, source, usage)
	if requestID != "" {
		res.Id = requestID
	}
	return res, nil
}

func (s *chatService) persistStreamContext(app *app, in *proto.ChatCompletionRequest, currMessage openai.ChatCompletionMessage, currTokens int, answer string, promptTokens, completionTokens int, replyID string) {
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
			ID:  replyID,
			PID: reqContext.ID,
			Message: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: answer,
			},
			Tokens: completionTokens,
		}
		if err := app.saveContext(resContext); err != nil {
			s.log.Error(err)
		}
	}()
}
