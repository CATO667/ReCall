package server

import (
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/services/tokenizer"
	"ai-chat-service/vecfluxapi/llm"
	"ai-chat-service/vecfluxapi/orchestrator"
	vpb "ai-chat-service/vecfluxapi/proto"
	"context"
	"io"

	"github.com/sashabaranov/go-openai"
)

func (s *VecFluxAPIServer) VGet(in *vpb.VGetRequest, stream vpb.VecFluxAPI_VGetServer) error {
	ctx := stream.Context()
	requestID := newRequestID()
	if in == nil || in.Question == "" {
		return stream.Send(&vpb.VGetChunk{RequestId: requestID, Done: true})
	}

	resolved, err := s.orchestrator.Resolve(ctx, in.Question)
	if err != nil {
		return err
	}

	if resolved.CacheHit && resolved.CacheRecord != nil {
		return s.streamCacheAnswer(stream, requestID, resolved.CacheRecord)
	}

	if err := s.sendHeartbeat(stream, requestID); err != nil {
		return err
	}

	ragRefs := resolved.RAGRefs
	source := vpb.ResponseSource_SOURCE_PURE_LLM
	if len(ragRefs) > 0 {
		source = vpb.ResponseSource_SOURCE_RAG_LLM
	}

	llmOpt := llm.OptionsFromConfig(s.cnf, in.Llm)
	req, promptTokens, err := s.llm.BuildRequest(in.Question, in.Context, ragRefs, llmOpt, true)
	if err != nil {
		return err
	}

	if err := stream.Send(&vpb.VGetChunk{
		RequestId: requestID,
		Source:    source,
		Refs:      toProtoRefs(ragRefs),
	}); err != nil {
		return err
	}

	chatStream, err := s.llm.CreateStream(ctx, req)
	if err != nil {
		return err
	}
	defer chatStream.Close()

	completion := ""
	resultID := ""
	finishReason := ""
	for {
		resp, err := chatStream.Recv()
		if err != nil && err != io.EOF {
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
		delta := llm.DeltaText(choice.Delta)
		if delta == "" {
			continue
		}
		completion += delta
		if err := stream.Send(&vpb.VGetChunk{
			RequestId: requestID,
			Source:    source,
			Delta:     delta,
		}); err != nil {
			return err
		}
	}

	model := llmOpt.Model
	resultMsg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: completion}
	completionTokens, err := tokenizer.GetTokens(&resultMsg, model)
	if err != nil {
		s.log.Error(err)
		completionTokens = 0
	}

	if err := stream.Send(&vpb.VGetChunk{
		RequestId:    requestID,
		Source:       source,
		FinishReason: finishReason,
		Done:         true,
		Refs:         toProtoRefs(ragRefs),
		Usage: &vpb.Usage{
			PromptTokens:     int32(promptTokens),
			CompletionTokens: int32(completionTokens),
			TotalTokens:      int32(promptTokens + completionTokens),
		},
	}); err != nil {
		return err
	}

	if llm.CompletionReadyForCache(ctx, finishReason, completion) {
		go func() {
			if _, err := s.persistQA(context.Background(), in.Question, completion, nil, promptTokens, completionTokens); err != nil {
				s.log.Error(err)
			}
		}()
	} else {
		s.log.InfoF("skip QA persist: finishReason=%q completionLen=%d ctxErr=%v", finishReason, len(completion), ctx.Err())
	}

	_ = resultID
	return nil
}

func (s *VecFluxAPIServer) streamCacheAnswer(stream vpb.VecFluxAPI_VGetServer, requestID string, rec *storage.QACacheRecord) error {
	tokensSaved := orchestrator.CacheTokensSaved(rec)
	usage := &vpb.Usage{TotalTokens: int32(rec.TotalTokens)}
	return s.streamTextChunks(stream, requestID, rec.Answer, vpb.ResponseSource_SOURCE_CACHE, tokensSaved, usage)
}

func toProtoRefs(refs []orchestrator.RAGRef) []*vpb.RAGRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]*vpb.RAGRef, 0, len(refs))
	for _, ref := range refs {
		out = append(out, &vpb.RAGRef{
			Question: ref.Question,
			Answer:   ref.Answer,
			Score:    ref.Score,
		})
	}
	return out
}
