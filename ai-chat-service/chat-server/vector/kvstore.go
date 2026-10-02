package vector

import (
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"fmt"
	"strings"
	"sync"
)

type kvstoreVectorStore struct {
	collection string
	dim        int
	indexType  string
	threshold  float32
	recallTopK int
	topK       int
	embedder   Embedder
	ensureOnce sync.Once
	ensureErr  error
}

func NewKvstoreVectorStore(cnf *config.Config, embedder Embedder) (VectorStore, error) {
	collection := cnf.Vector.Collection
	if collection == "" {
		collection = "ai_chat_qa"
	}
	dim := cnf.Vector.VectorDim
	if dim <= 0 {
		dim = 1024
	}
	threshold := cnf.Vector.SimilarityThreshold
	if threshold <= 0 {
		threshold = 0.4
	}
	recallTopK := cnf.Vector.RecallTopK
	if recallTopK <= 0 {
		recallTopK = 20
	}
	topK := cnf.Vector.TopK
	if topK <= 0 {
		topK = 3
	}
	indexType := strings.ToLower(strings.TrimSpace(cnf.Vector.IndexType))
	if indexType == "" {
		indexType = "hnsw"
	}
	return &kvstoreVectorStore{
		collection: collection,
		dim:        dim,
		indexType:  indexType,
		threshold:  threshold,
		recallTopK: recallTopK,
		topK:       topK,
		embedder:   embedder,
	}, nil
}

func (s *kvstoreVectorStore) ensureCollection(ctx context.Context) error {
	s.ensureOnce.Do(func() {
		if _, err := kvstore.VectorCount(ctx, s.collection); err == nil {
			return
		}
		s.ensureErr = kvstore.VectorCreate(ctx, s.collection, s.dim, "cosine", s.indexType)
	})
	return s.ensureErr
}

func (s *kvstoreVectorStore) Build(ctx context.Context, key string, question string, vector []float32) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	if len(vector) == 0 {
		var err error
		vector, err = s.embedder.Embed(ctx, question)
		if err != nil {
			return err
		}
	}
	if len(vector) == 0 {
		return fmt.Errorf("empty embedding vector")
	}
	if len(vector) != s.dim {
		return fmt.Errorf("embedding dim %d mismatch collection dim %d", len(vector), s.dim)
	}
	hash := storage.QuestionHash(question)
	return kvstore.VectorInsert(ctx, s.collection, hash, vector)
}

func (s *kvstoreVectorStore) SearchTopK(ctx context.Context, question string, vector []float32, topK int, scoreFilter ScoreFilter) ([]RetrievalHit, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	if len(vector) == 0 {
		var err error
		vector, err = s.embedder.Embed(ctx, question)
		if err != nil {
			return nil, err
		}
	}
	if len(vector) == 0 {
		return nil, nil
	}
	limit := topK
	if limit <= 0 {
		limit = s.topK
	}
	if s.recallTopK > limit {
		limit = s.recallTopK
	}
	raw, err := kvstore.VectorSearch(ctx, s.collection, limit, vector)
	if err != nil {
		return nil, err
	}
	finalK := topK
	if finalK <= 0 {
		finalK = s.topK
	}
	threshold, applyThreshold := scoreFilter.Resolve(s.threshold)
	hits := make([]RetrievalHit, 0, len(raw))
	for _, item := range raw {
		score := kvstore.CosineSimilarity(item.Distance)
		if applyThreshold && score < threshold {
			continue
		}
		qaKey := kvstore.QAMetaKey(item.ID)
		hits = append(hits, RetrievalHit{
			QAKey: qaKey,
			Score: score,
		})
		if len(hits) >= finalK {
			break
		}
	}
	return hits, nil
}
