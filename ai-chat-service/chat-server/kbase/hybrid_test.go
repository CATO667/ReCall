package kbase

import "testing"

func TestTokenizeTechnicalText(t *testing.T) {
	tokens := tokenize("mykvstore 支持 dict 和 跳表")
	joined := ""
	for _, t := range tokens {
		joined += t + "|"
	}
	if len(tokens) < 3 {
		t.Fatalf("expected multiple tokens, got %v", tokens)
	}
}

func TestKeywordOverlapScore(t *testing.T) {
	score := keywordOverlapScore("mykvstore WAL 快照", "持久化：WAL 增量日志 + SAVE 全量快照")
	if score < 0.3 {
		t.Fatalf("expected reasonable overlap, got %.2f", score)
	}
}

func TestMergeHybridHits(t *testing.T) {
	vec := []DocHit{{ChunkID: "a", Title: "t1", Content: "c1", Score: 0.8}}
	kw := []DocHit{{ChunkID: "a", Title: "t1", Content: "c1", Score: 0.6}, {ChunkID: "b", Title: "t2", Content: "c2", Score: 0.9}}
	merged := mergeHybridHits(vec, kw, 0.65, 0.35)
	if len(merged) != 2 {
		t.Fatalf("len=%d want 2", len(merged))
	}
	if merged[0].ChunkID != "a" {
		t.Fatalf("top chunk should be a, got %s", merged[0].ChunkID)
	}
}

func TestRerankCandidates(t *testing.T) {
	candidates := []RerankCandidate{
		{Source: "doc", Title: "intro", Content: "WAL 增量日志与全量快照", Score: 0.55},
		{Source: "doc", Title: "other", Content: "无关内容", Score: 0.70},
	}
	out := RerankCandidates("WAL 快照", candidates, 2, 0.15)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].Title != "intro" {
		t.Fatalf("keyword rerank should promote intro, got %s score=%.2f", out[0].Title, out[0].Score)
	}
}
