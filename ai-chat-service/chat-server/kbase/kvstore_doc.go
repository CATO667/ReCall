package kbase

import (
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type kvstoreChunkIndexer struct {
	collection string
	dim        int
	ensureOnce sync.Once
	ensureErr  error
	store      DocumentStore
}

type kvstoreDocSearcher struct {
	collection string
	threshold  float32
	recallTopK int
	topK       int
	store      DocumentStore
	ensureOnce sync.Once
	ensureErr  error
}

func newKvstoreChunkIndexer(cnf *config.Config) ChunkIndexer {
	collection := cnf.KBase.Collection
	if collection == "" {
		collection = "ai_chat_docs"
	}
	dim := cnf.Vector.VectorDim
	if dim <= 0 {
		dim = 1024
	}
	return &kvstoreChunkIndexer{
		collection: collection,
		dim:        dim,
		store:      NewDocumentStore(),
	}
}

func newKvstoreDocSearcher(cnf *config.Config) DocSearcher {
	collection := cnf.KBase.Collection
	if collection == "" {
		collection = "ai_chat_docs"
	}
	return &kvstoreDocSearcher{
		collection: collection,
		threshold:  docSearchThreshold(cnf),
		recallTopK: docRecallTopK(cnf),
		topK:       docTopK(cnf),
		store:      NewDocumentStore(),
	}
}

func (idx *kvstoreChunkIndexer) ensureCollection(ctx context.Context) error {
	idx.ensureOnce.Do(func() {
		if _, err := kvstore.VectorCount(ctx, idx.collection); err == nil {
			return
		}
		idx.ensureErr = kvstore.VectorCreate(ctx, idx.collection, idx.dim, "cosine", "flat")
	})
	return idx.ensureErr
}

func (idx *kvstoreChunkIndexer) IndexChunks(ctx context.Context, doc *DocumentMeta, chunks []Chunk, vectors [][]float32) error {
	if len(chunks) == 0 {
		return nil
	}
	if err := idx.ensureCollection(ctx); err != nil {
		return err
	}
	if len(vectors) != len(chunks) {
		return fmt.Errorf("vector count %d != chunk count %d", len(vectors), len(chunks))
	}
	for i, ch := range chunks {
		vec := vectors[i]
		if len(vec) != idx.dim {
			return fmt.Errorf("chunk %s vector dim %d mismatch %d", ch.ID, len(vec), idx.dim)
		}
		if err := kvstore.VectorInsert(ctx, idx.collection, ch.ID, vec); err != nil {
			return err
		}
	}
	return nil
}

func (idx *kvstoreChunkIndexer) DeleteDoc(ctx context.Context, docID string) error {
	if err := idx.ensureCollection(ctx); err != nil {
		return err
	}
	chunks, err := idx.store.GetChunks(ctx, docID)
	if err != nil {
		return err
	}
	for _, ch := range chunks {
		if err := kvstore.VectorDelete(ctx, idx.collection, ch.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *kvstoreDocSearcher) ensureCollection(ctx context.Context) error {
	s.ensureOnce.Do(func() {
		if _, err := kvstore.VectorCount(ctx, s.collection); err == nil {
			return
		}
		dim := 1024
		if cnf := config.GetConfig(); cnf != nil && cnf.Vector.VectorDim > 0 {
			dim = cnf.Vector.VectorDim
		}
		s.ensureErr = kvstore.VectorCreate(ctx, s.collection, dim, "cosine", "flat")
	})
	return s.ensureErr
}

func (s *kvstoreDocSearcher) Search(ctx context.Context, vec []float32, topK int) ([]DocHit, error) {
	if s == nil {
		return nil, nil
	}
	if len(vec) == 0 {
		return nil, fmt.Errorf("empty query vector")
	}
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	limit, finalK := docSearchLimits(topK, s.topK, s.recallTopK)
	raw, err := kvstore.VectorSearch(ctx, s.collection, limit, vec)
	if err != nil {
		return nil, err
	}
	hits := make([]DocHit, 0, len(raw))
	for _, item := range raw {
		hit, err := s.hydrateChunkHit(ctx, item.ID, kvstore.CosineSimilarity(item.Distance))
		if err != nil {
			return nil, err
		}
		if hit != nil {
			hits = append(hits, *hit)
		}
	}
	return filterDocHitsFromQdrant(hits, s.threshold, finalK), nil
}

func (s *kvstoreDocSearcher) hydrateChunkHit(ctx context.Context, chunkID string, score float32) (*DocHit, error) {
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	raw, err := client.Get(ctx, kvstore.ChunkKey(chunkID))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	ch := Chunk{}
	if err := json.Unmarshal(raw, &ch); err != nil {
		return nil, err
	}
	title := ""
	if ch.DocID != "" {
		meta, err := s.store.GetMeta(ctx, ch.DocID)
		if err != nil {
			return nil, err
		}
		if meta != nil {
			title = meta.Title
		}
	}
	return &DocHit{
		ChunkID: ch.ID,
		DocID:   ch.DocID,
		Title:   title,
		Chapter: ch.Chapter,
		Content: ch.Content,
		Score:   score,
	}, nil
}
