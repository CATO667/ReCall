package kbase

import (
	"context"
	"math"
	"sort"
	"sync"
)

type indexedChunk struct {
	hit DocHit
	tf  map[string]int
	dl  int
}

type KeywordIndex struct {
	mu      sync.RWMutex
	entries []indexedChunk
	idf     map[string]float64
	avgDL   float64
	loaded  bool
	store   DocumentStore
}

func NewKeywordIndex(store DocumentStore) *KeywordIndex {
	return &KeywordIndex{store: store, idf: make(map[string]float64)}
}

func (idx *KeywordIndex) ensureLoaded(ctx context.Context) error {
	idx.mu.RLock()
	if idx.loaded {
		idx.mu.RUnlock()
		return nil
	}
	idx.mu.RUnlock()

	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.loaded {
		return nil
	}
	metas, err := idx.store.ListMeta(ctx)
	if err != nil {
		return err
	}
	entries := make([]indexedChunk, 0, 256)
	df := make(map[string]int)
	for _, meta := range metas {
		chunks, err := idx.store.GetChunks(ctx, meta.ID)
		if err != nil {
			return err
		}
		for _, ch := range chunks {
			text := ch.Chapter + " " + meta.Title + " " + ch.Content
			tokens := tokenize(text)
			if len(tokens) == 0 {
				continue
			}
			tf := make(map[string]int, len(tokens))
			for _, t := range tokens {
				tf[t]++
			}
			entries = append(entries, indexedChunk{
				hit: DocHit{
					ChunkID: ch.ID,
					DocID:   ch.DocID,
					Title:   meta.Title,
					Chapter: ch.Chapter,
					Content: ch.Content,
				},
				tf: tf,
				dl: len(tokens),
			})
			seen := make(map[string]struct{})
			for t := range tf {
				if _, ok := seen[t]; ok {
					continue
				}
				seen[t] = struct{}{}
				df[t]++
			}
		}
	}
	n := float64(len(entries))
	if n == 0 {
		idx.loaded = true
		return nil
	}
	var totalDL float64
	for _, e := range entries {
		totalDL += float64(e.dl)
	}
	idf := make(map[string]float64, len(df))
	for term, freq := range df {
		idf[term] = math.Log(1 + (n-float64(freq)+0.5)/(float64(freq)+0.5))
	}
	idx.entries = entries
	idx.idf = idf
	idx.avgDL = totalDL / n
	idx.loaded = true
	return nil
}

func (idx *KeywordIndex) Invalidate() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.loaded = false
	idx.entries = nil
	idx.idf = make(map[string]float64)
	idx.avgDL = 0
}

func (idx *KeywordIndex) Search(ctx context.Context, query string, topK int) ([]DocHit, error) {
	if err := idx.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if len(idx.entries) == 0 {
		return nil, nil
	}
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return nil, nil
	}
	type scored struct {
		hit   DocHit
		score float64
	}
	scores := make([]scored, 0, len(idx.entries))
	k1, b := 1.2, 0.75
	for _, e := range idx.entries {
		var s float64
		for _, term := range qTokens {
			tf := float64(e.tf[term])
			if tf == 0 {
				continue
			}
			idf := idx.idf[term]
			dl := float64(e.dl)
			s += idf * (tf * (k1 + 1)) / (tf + k1*(1-b+b*dl/idx.avgDL))
		}
		if s <= 0 {
			continue
		}
		hit := e.hit
		hit.Score = float32(s)
		scores = append(scores, scored{hit: hit, score: s})
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	if topK <= 0 {
		topK = 10
	}
	if len(scores) > topK {
		scores = scores[:topK]
	}
	out := make([]DocHit, 0, len(scores))
	maxScore := scores[0].score
	for _, item := range scores {
		hit := item.hit
		if maxScore > 0 {
			hit.Score = float32(item.score / maxScore)
		}
		out = append(out, hit)
	}
	return out, nil
}
