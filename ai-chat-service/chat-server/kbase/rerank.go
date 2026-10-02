package kbase

import (
	"sort"
)

type RerankCandidate struct {
	Source   string
	Title    string
	Chapter  string
	Question string
	Answer   string
	Content  string
	Score    float32
	QAKey    string
}

func RerankCandidates(query string, candidates []RerankCandidate, topK int, keywordBoost float32) []RerankCandidate {
	if len(candidates) == 0 {
		return nil
	}
	if keywordBoost <= 0 {
		keywordBoost = 0.08
	}
	type ranked struct {
		ref         RerankCandidate
		rankedScore float32
	}
	rankedRefs := make([]ranked, 0, len(candidates))
	for _, ref := range candidates {
		text := ref.Content
		if ref.Source == "qa" {
			text = ref.Question + " " + ref.Answer
		}
		boost := keywordBoost * keywordOverlapScore(query, text)
		rankedRefs = append(rankedRefs, ranked{
			ref:         ref,
			rankedScore: ref.Score + boost,
		})
	}
	sort.Slice(rankedRefs, func(i, j int) bool {
		return rankedRefs[i].rankedScore > rankedRefs[j].rankedScore
	})
	if topK <= 0 {
		topK = 3
	}
	if len(rankedRefs) > topK {
		rankedRefs = rankedRefs[:topK]
	}
	out := make([]RerankCandidate, 0, len(rankedRefs))
	for _, item := range rankedRefs {
		ref := item.ref
		ref.Score = item.rankedScore
		out = append(out, ref)
	}
	return out
}
