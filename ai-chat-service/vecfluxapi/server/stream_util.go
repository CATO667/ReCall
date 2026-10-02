package server

import vpb "ai-chat-service/vecfluxapi/proto"

const (
	streamChunkRunes      = 64
	finishReasonHeartbeat = "heartbeat"
	heartbeatText         = "检索中…"
)

func splitTextRunes(text string, size int) []string {
	if size <= 0 {
		size = streamChunkRunes
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

func (s *VecFluxAPIServer) sendHeartbeat(stream vpb.VecFluxAPI_VGetServer, requestID string) error {
	return stream.Send(&vpb.VGetChunk{
		RequestId:    requestID,
		Delta:        heartbeatText,
		FinishReason: finishReasonHeartbeat,
	})
}

func (s *VecFluxAPIServer) streamTextChunks(stream vpb.VecFluxAPI_VGetServer, requestID, text string, source vpb.ResponseSource, tokensSaved int32, usage *vpb.Usage) error {
	chunks := splitTextRunes(text, streamChunkRunes)
	if len(chunks) == 0 {
		return stream.Send(&vpb.VGetChunk{
			RequestId:    requestID,
			Source:       source,
			FinishReason: "stop",
			Done:         true,
			TokensSaved:  tokensSaved,
			Usage:        usage,
		})
	}
	for _, chunk := range chunks {
		if err := stream.Send(&vpb.VGetChunk{
			RequestId: requestID,
			Source:    source,
			Delta:     chunk,
		}); err != nil {
			return err
		}
	}
	return stream.Send(&vpb.VGetChunk{
		RequestId:    requestID,
		Source:       source,
		FinishReason: "stop",
		Done:         true,
		TokensSaved:  tokensSaved,
		Usage:        usage,
	})
}
