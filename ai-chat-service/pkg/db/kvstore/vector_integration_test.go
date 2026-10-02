//go:build integration

package kvstore_test

import (
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "kvstore-int-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	cfg := filepath.Join(tmp, "test.config.yaml")
	content := []byte(`kvstore:
  write_host: "127.0.0.1"
  read_host: "127.0.0.1"
  port: 2000
  read_port: 2000
  prefix: "test_"
vector:
  enabled: true
  provider: "kvstore"
  collection: "ai_chat_qa"
  vector_dim: 3
kbase:
  enabled: true
  collection: "ai_chat_docs"
`)
	if err := os.WriteFile(cfg, content, 0644); err != nil {
		panic(err)
	}
	config.InitConfig(cfg)
	kvstore.InitPools(config.GetConfig())
	os.Exit(m.Run())
}

func TestVectorCreateInsertSearch(t *testing.T) {
	ctx := context.Background()
	const coll = "go_int_test"
	_ = kvstore.VectorDrop(ctx, coll)
	if err := kvstore.VectorCreate(ctx, coll, 3, "cosine", "flat"); err != nil {
		t.Fatal(err)
	}
	if err := kvstore.VectorInsert(ctx, coll, "a", []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := kvstore.VectorInsert(ctx, coll, "b", []float32{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	hits, err := kvstore.VectorSearch(ctx, coll, 2, []float32{1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 1 || hits[0].ID != "a" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
	sim := kvstore.CosineSimilarity(hits[0].Distance)
	if sim < 0.99 {
		t.Fatalf("expected high similarity, got %f", sim)
	}
	count, err := kvstore.VectorCount(ctx, coll)
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := kvstore.VectorDelete(ctx, coll, "b"); err != nil {
		t.Fatal(err)
	}
	count, err = kvstore.VectorCount(ctx, coll)
	if err != nil || count != 1 {
		t.Fatalf("after delete count=%d err=%v", count, err)
	}
	_ = kvstore.VectorDrop(ctx, coll)
}
