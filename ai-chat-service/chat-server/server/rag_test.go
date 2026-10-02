package server

import (
	"ai-chat-service/chat-server/rerank"
	"ai-chat-service/pkg/config"
	"testing"
)

func TestPrepareQueryText(t *testing.T) {
	got := prepareQueryText("  什么是  RAG？  ")
	want := "什么是 rag？"
	if got != want {
		t.Fatalf("prepareQueryText() = %q, want %q", got, want)
	}

	got = prepareQueryText("  用 Golang 写代码  ")
	want = "用 go 写代码"
	if got != want {
		t.Fatalf("prepareQueryText() lang alias = %q, want %q", got, want)
	}
}

func TestRagRefsFromResults(t *testing.T) {
	results := []rerank.Result{
		{Candidate: rerank.Candidate{QAKey: "qa1", Question: "q1", Answer: "a1"}, FinalScore: 0.55},
		{Candidate: rerank.Candidate{QAKey: "qa2", Question: "q2", Answer: "a2"}, FinalScore: 0.45},
		{Candidate: rerank.Candidate{QAKey: "qa3", Question: "q3", Answer: "a3"}, FinalScore: 0.35},
	}
	refs := ragRefsFromResults(results, 0.4, 2)
	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2", len(refs))
	}
	if refs[0].QAKey != "qa1" || refs[1].QAKey != "qa2" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestSemanticThresholdDefaults(t *testing.T) {
	s := &chatService{config: &config.Config{}}
	if got := s.semanticCacheThreshold(); got != 0.65 {
		t.Fatalf("semanticCacheThreshold default = %.2f, want 0.65", got)
	}
	if got := s.ragSimilarityThreshold(); got != 0.4 {
		t.Fatalf("ragSimilarityThreshold default = %.2f, want 0.4", got)
	}
}

func TestBuildRAGContextBlock_QAOnly(t *testing.T) {
	block := buildRAGContextBlock([]RAGRef{
		{Source: "qa", Question: "什么是 Redis", Answer: "内存数据库", Score: 0.82},
	})
	if block == "" {
		t.Fatal("expected non-empty rag block")
	}
	if !containsAll(block, "历史问答", "什么是 Redis", "内存数据库") {
		t.Fatalf("unexpected rag block: %s", block)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
