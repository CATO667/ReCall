package rerank

import (
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/pkg/config"
	"sort"
	"strings"
)

type Service struct {
	cfg Config
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg.withDefaults()}
}

func NewServiceFromConfig(cnf *config.Config) *Service {
	cfg := Config{Enabled: true}
	if cnf != nil {
		r := cnf.Rerank
		unset := !r.Enabled && r.WeightVector == 0 && r.WeightLexical == 0 && r.WeightTopic == 0
		cfg.Enabled = unset || r.Enabled
		cfg.Weights = Weights{
			Vector:  r.WeightVector,
			Lexical: r.WeightLexical,
			Topic:   r.WeightTopic,
		}
	}
	return NewService(cfg)
}

func (c Config) withDefaults() Config {
	out := c
	if out.Weights.Vector <= 0 {
		out.Weights.Vector = 0.75
	}
	if out.Weights.Lexical <= 0 {
		out.Weights.Lexical = 0.20
	}
	if out.Weights.Topic <= 0 {
		out.Weights.Topic = 0.05
	}
	return out
}

func GuessQueryTopicID(candidates []Candidate, minCount int) string {
	if minCount <= 0 {
		minCount = 2
	}
	counts := make(map[string]int)
	for _, c := range candidates {
		if c.TopicID == "" {
			continue
		}
		counts[c.TopicID]++
	}
	bestID := ""
	bestN := 0
	for id, n := range counts {
		if n > bestN {
			bestN = n
			bestID = id
		}
	}
	if bestN >= minCount {
		return bestID
	}
	return ""
}

func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled
}

// Rerank applies L0 rule filtering and L1 local scoring, then returns topK results.
func (s *Service) Rerank(req Request) []Result {
	if s == nil || !s.cfg.Enabled || len(req.Candidates) == 0 {
		return nil
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 3
	}

	queryTopicID := req.QueryTopicID
	if queryTopicID == "" {
		queryTopicID = GuessQueryTopicID(req.Candidates, 2)
	}

	normalizedQuery := req.NormalizedQuery
	if normalizedQuery == "" {
		normalizedQuery = storage.PrepareQueryText(req.RawQuery)
	}

	filtered := s.filterL0(req.RawQuery, normalizedQuery, req.Candidates)
	if len(filtered) == 0 {
		return nil
	}

	scored := make([]Result, 0, len(filtered))
	for _, cand := range filtered {
		result := s.scoreL1(req.RawQuery, queryTopicID, cand)
		scored = append(scored, result)
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].FinalScore == scored[j].FinalScore {
			return scored[i].VectorScore > scored[j].VectorScore
		}
		return scored[i].FinalScore > scored[j].FinalScore
	})

	if len(scored) > topK {
		scored = scored[:topK]
	}
	return scored
}

func (s *Service) filterL0(rawQuery, normalizedQuery string, candidates []Candidate) []Candidate {
	out := make([]Candidate, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, cand := range candidates {
		if cand.QAKey != "" {
			if _, ok := seen[cand.QAKey]; ok {
				continue
			}
		}
		if strings.TrimSpace(cand.Answer) == "" {
			continue
		}
		if !storage.LanguagesCompatible(rawQuery, cand.Question) {
			continue
		}
		if !storage.SubjectsCompatible(rawQuery, cand.Question) {
			continue
		}
		if cand.QAKey != "" {
			seen[cand.QAKey] = struct{}{}
		}
		out = append(out, cand)
	}
	return out
}

func (s *Service) scoreL1(rawQuery, queryTopicID string, cand Candidate) Result {
	lexical := lexicalScore(rawQuery, cand.Question)
	topicBoost := float32(0)
	if queryTopicID != "" && cand.TopicID != "" && queryTopicID == cand.TopicID {
		topicBoost = s.cfg.Weights.Topic
	}
	final := s.cfg.Weights.Vector*cand.VectorScore +
		s.cfg.Weights.Lexical*lexical +
		topicBoost
	if storage.QuestionsEquivalent(rawQuery, cand.Question) {
		if final < 1.0 {
			final = 1.0
		}
		lexical = 1.0
	}
	return Result{
		Candidate:    cand,
		FinalScore:   final,
		LexicalScore: lexical,
		TopicBoost:   topicBoost,
	}
}
