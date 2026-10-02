package server

import (
	"ai-chat-service/proto"
	"context"
	"strings"
)

const (
	StreamChunkRunes      = 64
	FinishReasonHeartbeat = "heartbeat"
	HeartbeatText         = "检索中…"
)

func SplitTextRunes(text string, size int) []string {
	if size <= 0 {
		size = StreamChunkRunes
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	out := make([]string, 0, (len(runes)+size-1)/size)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}

func (a *app) streamTextChunks(stream proto.Chat_ChatCompletionStreamServer, id, text string, source proto.ResponseSource, usage *proto.Usage, tokensSaved int32) error {
	chunks := SplitTextRunes(text, StreamChunkRunes)
	if len(chunks) == 0 {
		return stream.Send(a.buildChatCompletionStreamResponse(id, "", "stop", source, usage, tokensSaved))
	}
	for _, chunk := range chunks {
		if err := stream.Send(a.buildChatCompletionStreamResponse(id, chunk, "", source, nil, 0)); err != nil {
			return err
		}
	}
	return stream.Send(a.buildChatCompletionStreamResponse(id, "", "stop", source, usage, tokensSaved))
}

func (a *app) streamHeartbeat(stream proto.Chat_ChatCompletionStreamServer, id string) error {
	return stream.Send(a.buildChatCompletionStreamResponse(id, HeartbeatText, FinishReasonHeartbeat, proto.ResponseSource_SOURCE_UNSPECIFIED, nil, 0))
}

func completionReadyForCache(ctx context.Context, finishReason, completion string) bool {
	if ctx.Err() != nil {
		return false
	}
	if strings.TrimSpace(completion) == "" {
		return false
	}
	switch finishReason {
	case "stop", "length":
		return true
	default:
		return false
	}
}
