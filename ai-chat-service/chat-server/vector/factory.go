package vector

import (
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/pkg/config"
	"log"
)

func newEmbedderFromConfig(cnf *config.Config) Embedder {
	provider := cnf.Embedder.Provider
	if provider == "" {
		provider = "noop"
	}
	switch provider {
	case "siliconflow", "openai_compat", "ollama":
		return NewSiliconFlowEmbedder(cnf)
	default:
		return NewNoopEmbedder()
	}
}

func NewFromConfig(cnf *config.Config, qaStore storage.IQAStore) (Embedder, VectorStore) {
	if !cnf.Vector.Enabled {
		return NewNoopEmbedder(), NewNoopVectorStore()
	}
	embedder := newEmbedderFromConfig(cnf)
	switch ProviderName(cnf) {
	case ProviderKvstore:
		store, err := NewKvstoreVectorStore(cnf, embedder)
		if err != nil {
			log.Fatalf("init kvstore vector store: %v", err)
		}
		return embedder, store
	default:
		log.Fatalf("unsupported vector provider %q (only kvstore is supported)", ProviderName(cnf))
		return embedder, NewNoopVectorStore()
	}
}
