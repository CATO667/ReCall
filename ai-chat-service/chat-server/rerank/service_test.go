package rerank

import "testing"

func TestRerank_L0Filters(t *testing.T) {
	s := NewService(Config{Enabled: true})
	out := s.Rerank(Request{
		RawQuery:        "mykvstore 主从怎么配置",
		NormalizedQuery: "mykvstore 主从怎么配置",
		Candidates: []Candidate{
			{QAKey: "qa1", Question: "redis 主从怎么配置", Answer: "a1", VectorScore: 0.9},
			{QAKey: "qa2", Question: "mykvstore 主从怎么配置", Answer: "a2", VectorScore: 0.95},
			{QAKey: "qa3", Question: "mykvstore 持久化方案", Answer: "", VectorScore: 0.99},
		},
		TopK: 3,
	})
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2 after L0 filters", len(out))
	}
	if out[0].QAKey != "qa2" {
		t.Fatalf("top=%q, want exact match qa2", out[0].QAKey)
	}
	if out[0].FinalScore < 1.0 {
		t.Fatalf("exact match final score = %.2f, want 1.0", out[0].FinalScore)
	}
}

func TestRerank_L1LexicalBoost(t *testing.T) {
	s := NewService(Config{
		Enabled: true,
		Weights: Weights{Vector: 0.75, Lexical: 0.20, Topic: 0.05},
	})
	out := s.Rerank(Request{
		RawQuery: "WAL 快照怎么恢复",
		Candidates: []Candidate{
			{QAKey: "qa1", Question: "无关问题", Answer: "a1", VectorScore: 0.70},
			{QAKey: "qa2", Question: "WAL 快照恢复流程", Answer: "a2", VectorScore: 0.68},
		},
		TopK: 2,
	})
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].QAKey != "qa2" {
		t.Fatalf("top=%q, want qa2 boosted by lexical overlap", out[0].QAKey)
	}
}

func TestRerank_TopicBoost(t *testing.T) {
	s := NewService(Config{
		Enabled: true,
		Weights: Weights{Vector: 0.75, Lexical: 0.20, Topic: 0.05},
	})
	out := s.Rerank(Request{
		RawQuery:     "redis 持久化",
		QueryTopicID: "topic-a",
		Candidates: []Candidate{
			{QAKey: "qa1", Question: "q1", Answer: "a1", VectorScore: 0.70},
			{QAKey: "qa2", Question: "q2", Answer: "a2", TopicID: "topic-a", VectorScore: 0.69},
		},
		TopK: 2,
	})
	if out[0].QAKey != "qa2" {
		t.Fatalf("top=%q, want qa2 with topic boost", out[0].QAKey)
	}
	if out[0].TopicBoost != 0.05 {
		t.Fatalf("topic boost = %.2f, want 0.05", out[0].TopicBoost)
	}
}

func TestRerank_EquivalentQuestionBoost(t *testing.T) {
	s := NewService(Config{Enabled: true})
	out := s.Rerank(Request{
		RawQuery: "c语言生成b+tree",
		Candidates: []Candidate{
			{QAKey: "qa1", Question: "c语言生成b+树", Answer: "bplus", VectorScore: 0.72},
		},
		TopK: 1,
	})
	if len(out) != 1 || out[0].FinalScore < 1.0 {
		t.Fatalf("equivalent question boost out = %+v, want final score 1.0", out)
	}
}

func TestRerank_L0SubjectFilter(t *testing.T) {
	s := NewService(Config{Enabled: true})
	out := s.Rerank(Request{
		RawQuery: "c语言生成b+树",
		Candidates: []Candidate{
			{QAKey: "qa1", Question: "c语言生成红黑树", Answer: "rbtree", VectorScore: 0.91},
			{QAKey: "qa2", Question: "c语言生成b+树", Answer: "bplus", VectorScore: 0.70},
		},
		TopK: 2,
	})
	if len(out) != 1 || out[0].QAKey != "qa2" {
		t.Fatalf("subject filter out = %+v, want only qa2", out)
	}
}

func TestGuessQueryTopicID(t *testing.T) {
	got := GuessQueryTopicID([]Candidate{
		{TopicID: "t1"},
		{TopicID: "t1"},
		{TopicID: "t2"},
	}, 2)
	if got != "t1" {
		t.Fatalf("GuessQueryTopicID() = %q, want t1", got)
	}
}
