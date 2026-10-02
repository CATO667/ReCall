package kbase

import (
	"ai-chat-service/pkg/config"
	"context"
	"sync"
)

type HybridSearcher interface {
	DocSearcher
	SearchHybrid(ctx context.Context, query string, vector []float32, topK int) ([]DocHit, error)
}

type hybridDocSearcher struct {
	inner   DocSearcher
	index   *KeywordIndex
	vecW    float32
	kwW     float32
	recallK int
}

var (
	activeKeywordIndex *KeywordIndex
	activeIndexMu      sync.Mutex
)

func registerKeywordIndex(idx *KeywordIndex) {
	activeIndexMu.Lock()
	activeKeywordIndex = idx
	activeIndexMu.Unlock()
}

func InvalidateKeywordIndex() {
	activeIndexMu.Lock()
	idx := activeKeywordIndex
	activeIndexMu.Unlock()
	if idx != nil {
		idx.Invalidate()
	}
}

func NewHybridDocSearcher(cnf *config.Config, inner DocSearcher, store DocumentStore) DocSearcher {
	if inner == nil || cnf == nil || !cnf.KBase.HybridEnabled {
		return inner
	}
	vecW := cnf.KBase.HybridVectorWeight
	kwW := cnf.KBase.HybridKeywordWeight
	if vecW <= 0 {
		vecW = 0.65
	}
	if kwW <= 0 {
		kwW = 0.35
	}
	recallK := cnf.Vector.RecallTopK
	if recallK <= 0 {
		recallK = 20
	}
	idx := NewKeywordIndex(store)
	registerKeywordIndex(idx)
	return &hybridDocSearcher{
		inner:   inner,
		index:   idx,
		vecW:    vecW,
		kwW:     kwW,
		recallK: recallK,
	}
}

func (h *hybridDocSearcher) Search(ctx context.Context, vector []float32, topK int) ([]DocHit, error) {
	return h.inner.Search(ctx, vector, topK)
}

func (h *hybridDocSearcher) SearchHybrid(ctx context.Context, query string, vector []float32, topK int) ([]DocHit, error) {
	recall := h.recallK
	if topK > 0 && recall < topK*3 {
		recall = topK * 3
	}
	vecHits, err := h.inner.Search(ctx, vector, recall)
	if err != nil {
		return nil, err
	}
	kwHits, err := h.index.Search(ctx, query, recall)
	if err != nil {
		return vecHits, nil
	}
	merged := mergeHybridHits(vecHits, kwHits, h.vecW, h.kwW)
	if topK <= 0 {
		topK = 3
	}
	if len(merged) > topK {
		merged = merged[:topK]
	}
	return merged, nil
}

func mergeHybridHits(vecHits, kwHits []DocHit, vecW, kwW float32) []DocHit {
	type item struct {
		hit   DocHit
		score float32
	}
	byID := make(map[string]item)
	update := func(hit DocHit, part float32) {
		if hit.ChunkID == "" {
			return
		}
		if cur, ok := byID[hit.ChunkID]; ok {
			cur.score += part
			if hit.Content != "" {
				cur.hit = hit
			}
			byID[hit.ChunkID] = cur
			return
		}
		byID[hit.ChunkID] = item{hit: hit, score: part}
	}
	for _, hit := range vecHits {
		update(hit, vecW*hit.Score)
	}
	for _, hit := range kwHits {
		update(hit, kwW*hit.Score)
	}
	out := make([]DocHit, 0, len(byID))
	for _, v := range byID {
		hit := v.hit
		hit.Score = v.score
		out = append(out, hit)
	}
	sortDocHits(out)
	return out
}

func sortDocHits(hits []DocHit) {
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].Score > hits[i].Score {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
}
