package vector_test

import (
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func ollamaReachable() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:11434/")
	return err == nil && resp != nil && resp.Body.Close() == nil
}

func TestOllamaEmbedder(t *testing.T) {
	if os.Getenv("SKIP_OLLAMA_TEST") == "1" || !ollamaReachable() {
		t.Skip("ollama not reachable at http://127.0.0.1:11434")
	}
	cnf := &config.Config{}
	cnf.Embedder.Provider = "openai_compat"
	cnf.Embedder.ApiKey = "ollama"
	cnf.Embedder.BaseUrl = "http://127.0.0.1:11434/v1"
	cnf.Embedder.Model = "bge-m3"
	emb := vector.NewSiliconFlowEmbedder(cnf)
	start := time.Now()
	vec, err := emb.Embed(context.Background(), "什么是Redis")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 1024 {
		t.Fatalf("expected 1024 dims, got %d", len(vec))
	}
	t.Logf("ollama embed ok dim=%d elapsed=%v", len(vec), time.Since(start))
}
