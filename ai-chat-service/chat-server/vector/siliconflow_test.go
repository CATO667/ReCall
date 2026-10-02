package vector_test

import (
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"context"
	"os"
	"testing"
)

func TestSiliconFlowEmbedder(t *testing.T) {
	key := os.Getenv("SILICONFLOW_API_KEY")
	if key == "" {
		t.Skip("SILICONFLOW_API_KEY not set")
	}
	cnf := &config.Config{}
	cnf.Embedder.Provider = "siliconflow"
	cnf.Embedder.ApiKey = key
	cnf.Embedder.BaseUrl = "https://api.siliconflow.cn/v1"
	cnf.Embedder.Model = "BAAI/bge-m3"
	emb := vector.NewSiliconFlowEmbedder(cnf)
	vec, err := emb.Embed(context.Background(), "什么是Redis")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 1024 {
		t.Fatalf("expected 1024 dims, got %d", len(vec))
	}
}
