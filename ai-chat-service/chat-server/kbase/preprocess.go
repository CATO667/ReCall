package kbase

import (
	"regexp"
	"strings"
)

var (
	adLineRe      = regexp.MustCompile(`(?i)(soushu|搜书吧|精校吧|www\.soushu)`)
	separatorRe   = regexp.MustCompile(`^={3,}\s*$`)
	fullWidthLead = regexp.MustCompile(`^[\s　]+`)
)

func PreprocessNovel(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(out) > 0 && out[len(out)-1] == "" {
				continue
			}
			out = append(out, "")
			continue
		}
		if separatorRe.MatchString(trimmed) {
			continue
		}
		if adLineRe.MatchString(trimmed) {
			continue
		}
		cleaned := fullWidthLead.ReplaceAllString(line, "")
		out = append(out, strings.TrimSpace(cleaned))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func PreprocessContent(content, sourceType string) string {
	if sourceType == SourceTypeText {
		return PreprocessNovel(content)
	}
	return content
}
