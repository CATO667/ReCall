package rerank

import (
	"strings"
	"unicode"
)

func tokenize(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}
	var tokens []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		tokens = append(tokens, buf.String())
		buf.Reset()
	}
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			buf.WriteRune(r)
		case r >= 0x4e00 && r <= 0x9fff:
			flush()
			tokens = append(tokens, string(r))
		default:
			flush()
		}
	}
	flush()

	runes := []rune(text)
	for i := 0; i+1 < len(runes); i++ {
		a, b := runes[i], runes[i+1]
		if isCJK(a) && isCJK(b) {
			tokens = append(tokens, string([]rune{a, b}))
		}
	}
	return dedupeTokens(tokens)
}

func isCJK(r rune) bool {
	return r >= 0x4e00 && r <= 0x9fff
}

func dedupeTokens(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		if len(t) < 2 && (len(t) == 0 || !isCJK(rune(t[0]))) {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func lexicalScore(query, question string) float32 {
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return 0
	}
	questionLower := strings.ToLower(question)
	hits := 0
	for _, t := range qTokens {
		if strings.Contains(questionLower, t) {
			hits++
		}
	}
	return float32(hits) / float32(len(qTokens))
}
