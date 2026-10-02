package kbase

import (
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"context"
)

type DocHit struct {
	ChunkID string
	DocID   string
	Title   string
	Chapter string
	Content string
	Score   float32
}

type DocSearcher interface {
	Search(ctx context.Context, vector []float32, topK int) ([]DocHit, error)
}

func NewDocSearcher(cnf *config.Config) DocSearcher {
	if cnf == nil || !cnf.KBase.Enabled || !cnf.Vector.Enabled {
		return nil
	}
	if vector.ProviderName(cnf) != vector.ProviderKvstore {
		return nil
	}
	return newKvstoreDocSearcher(cnf)
}

func docSearchThreshold(cnf *config.Config) float32 {
	if cnf.KBase.SimilarityThreshold > 0 {
		return cnf.KBase.SimilarityThreshold
	}
	if cnf.Vector.SimilarityThreshold > 0 {
		return cnf.Vector.SimilarityThreshold
	}
	return 0.4
}

func docRecallTopK(cnf *config.Config) int {
	if cnf.Vector.RecallTopK > 0 {
		return cnf.Vector.RecallTopK
	}
	return 20
}

func docTopK(cnf *config.Config) int {
	if cnf.KBase.RerankEnabled && cnf.Vector.TopK > 0 {
		return cnf.Vector.TopK
	}
	if cnf.Vector.TopK > 0 {
		return cnf.Vector.TopK
	}
	return 5
}

func docSearchLimits(topK, defaultTopK, recallTopK int) (limit, finalK int) {
	limit = topK
	if limit <= 0 {
		limit = defaultTopK
	}
	if recallTopK > limit {
		limit = recallTopK
	}
	finalK = topK
	if finalK <= 0 {
		finalK = defaultTopK
	}
	return limit, finalK
}

func filterDocHitsFromQdrant(raw []DocHit, threshold float32, finalK int) []DocHit {
	out := make([]DocHit, 0, len(raw))
	for _, item := range raw {
		if item.Score < threshold {
			continue
		}
		out = append(out, item)
		if len(out) >= finalK {
			break
		}
	}
	return out
}
