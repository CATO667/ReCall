package storage

import (
	"regexp"
	"sort"
	"strings"
)

// langAlias maps common spellings to canonical language ids used for cache gating.
var langAlias = map[string]string{
	"golang":      "go",
	"cpp":         "c++",
	"c plus plus": "c++",
	"cplusplus":   "c++",
	"py":          "python",
	"js":          "javascript",
	"ts":          "typescript",
	"csharp":      "c#",
	"c sharp":     "c#",
	"cs":          "c#",
}

// langAliasPatterns are applied longest-first so "c plus plus" wins over "c".
var langAliasPatterns []aliasPattern

type aliasPattern struct {
	from string
	to   string
}

func init() {
	seen := make(map[string]struct{}, len(langAlias))
	for from, to := range langAlias {
		seen[from] = struct{}{}
		langAliasPatterns = append(langAliasPatterns, aliasPattern{from: from, to: to})
	}
	sort.Slice(langAliasPatterns, func(i, j int) bool {
		return len(langAliasPatterns[i].from) > len(langAliasPatterns[j].from)
	})
	_ = seen
}

type langEntry struct {
	id       string
	literals []string
}

// langEntries is ordered longest-id-first; c++ / c# before c.
var langEntries = []langEntry{
	{id: "javascript", literals: []string{"javascript", "javascript语言"}},
	{id: "typescript", literals: []string{"typescript", "typescript语言"}},
	{id: "python", literals: []string{"python", "python语言"}},
	{id: "kotlin", literals: []string{"kotlin", "kotlin语言"}},
	{id: "swift", literals: []string{"swift", "swift语言"}},
	{id: "scala", literals: []string{"scala", "scala语言"}},
	{id: "ruby", literals: []string{"ruby", "ruby语言"}},
	{id: "rust", literals: []string{"rust", "rust语言"}},
	{id: "java", literals: []string{"java", "java语言"}},
	{id: "lua", literals: []string{"lua", "lua语言"}},
	{id: "php", literals: []string{"php", "php语言"}},
	{id: "c++", literals: []string{"c++", "c++语言"}},
	{id: "c#", literals: []string{"c#", "c#语言"}},
	{id: "go", literals: []string{"go语言"}},
	{id: "c", literals: []string{"c语言"}},
}

var (
	goWordRe = regexp.MustCompile(`(?:^|[^a-z0-9_])go(?:$|[^a-z0-9_])`)
	cWordRe  = regexp.MustCompile(`(?:^|[^a-z0-9_+#])c(?:$|[^a-z0-9_+#])`)
)

// PrepareQueryText normalizes whitespace/case and canonicalizes language/subject tokens for embed/hash.
func PrepareQueryText(q string) string {
	q = NormalizeQuestion(q)
	q = ApplyLanguageAliases(q)
	return ApplySubjectTermAliases(q)
}

// ApplyLanguageAliases replaces known language spellings with canonical ids.
func ApplyLanguageAliases(q string) string {
	for _, item := range langAliasPatterns {
		q = replaceWholePhrase(q, item.from, item.to)
	}
	return q
}

func replaceWholePhrase(text, from, to string) string {
	if from == "" || !strings.Contains(text, from) {
		return text
	}
	re := regexp.MustCompile(`(?i)(^|[^a-z0-9_+#])` + regexp.QuoteMeta(from) + `($|[^a-z0-9_+#])`)
	return re.ReplaceAllString(text, `${1}`+to+`${2}`)
}

// ExtractLanguages returns canonical language ids mentioned in the question.
func ExtractLanguages(q string) []string {
	q = ApplyLanguageAliases(NormalizeQuestion(q))
	found := map[string]struct{}{}
	masked := q

	for _, entry := range langEntries {
		for _, lit := range entry.literals {
			if idx := strings.Index(masked, lit); idx >= 0 {
				found[entry.id] = struct{}{}
				masked = masked[:idx] + strings.Repeat(" ", len(lit)) + masked[idx+len(lit):]
				break
			}
		}
	}
	if goWordRe.MatchString(masked) {
		found["go"] = struct{}{}
	}
	if cWordRe.MatchString(masked) {
		found["c"] = struct{}{}
	}

	out := make([]string, 0, len(found))
	for id := range found {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// LanguagesCompatible reports whether two questions may share semantic QA cache.
// When both mention languages, at least one canonical id must overlap.
func LanguagesCompatible(q1, q2 string) bool {
	l1 := ExtractLanguages(q1)
	l2 := ExtractLanguages(q2)
	if len(l1) == 0 || len(l2) == 0 {
		return true
	}
	for _, a := range l1 {
		for _, b := range l2 {
			if a == b {
				return true
			}
		}
	}
	return false
}
