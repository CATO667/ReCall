package vector

import "context"

type noopEmbedder struct{}

func NewNoopEmbedder() Embedder {
	return &noopEmbedder{}
}

func (e *noopEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

type noopVectorStore struct{}

func NewNoopVectorStore() VectorStore {
	return &noopVectorStore{}
}

func (s *noopVectorStore) Build(ctx context.Context, key string, question string, vector []float32) error {
	return nil
}

func (s *noopVectorStore) SearchTopK(ctx context.Context, question string, vector []float32, topK int, scoreFilter ScoreFilter) ([]RetrievalHit, error) {
	return nil, nil
}
