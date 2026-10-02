package storage

import (
	"regexp"
	"sort"
	"strings"
)

type subjectPattern struct {
	id string
	re *regexp.Regexp
}

// subjectPatterns are matched longest-id-first to avoid b+树 being swallowed by shorter ids.
var subjectPatterns []subjectPattern

func init() {
	raw := []struct {
		id       string
		patterns []string
	}{
		{id: "b+树", patterns: []string{`b\s*\+\s*树`, `b\s*\+\s*tree`, `bplus\s*树?`, `b\s*plus\s*tree`}},
		{id: "b树", patterns: []string{`b\s*树`}},
		{id: "红黑树", patterns: []string{`红黑树`, `\brbtree\b`, `red\s*black\s*tree`}},
		{id: "avl树", patterns: []string{`avl\s*树?`, `\navl\b`}},
		{id: "跳表", patterns: []string{`跳表`, `\bskip\s*list\b`, `\bskiplist\b`}},
		{id: "二叉搜索树", patterns: []string{`二叉搜索树`, `\bbst\b`}},
		{id: "二叉树", patterns: []string{`二叉树`}},
		{id: "平衡树", patterns: []string{`平衡树`, `平衡二叉树`}},
		{id: "字典树", patterns: []string{`字典树`, `\btrie\b`}},
		{id: "链表", patterns: []string{`链表`, `\blinked\s*list\b`}},
		{id: "哈希表", patterns: []string{`哈希表`, `hash\s*map`, `\bhashmap\b`}},
		{id: "堆", patterns: []string{`最小堆`, `最大堆`, `优先队列`}},
	}
	sort.Slice(raw, func(i, j int) bool { return len(raw[i].id) > len(raw[j].id) })
	for _, item := range raw {
		for _, p := range item.patterns {
			subjectPatterns = append(subjectPatterns, subjectPattern{
				id: item.id,
				re: regexp.MustCompile(`(?i)` + p),
			})
		}
	}
}

var subjectTermAliasPatterns []struct {
	re *regexp.Regexp
	to string
}

func init() {
	subjectTermAliasPatterns = []struct {
		re *regexp.Regexp
		to string
	}{
		{regexp.MustCompile(`(?i)b\s*\+\s*tree`), "b+树"},
		{regexp.MustCompile(`(?i)b\s*plus\s*tree`), "b+树"},
		{regexp.MustCompile(`(?i)red\s*black\s*tree`), "红黑树"},
		{regexp.MustCompile(`(?i)\brbtree\b`), "红黑树"},
		{regexp.MustCompile(`(?i)\bavl\s*tree\b`), "avl树"},
		{regexp.MustCompile(`(?i)\bskip\s*list\b`), "跳表"},
		{regexp.MustCompile(`(?i)\bskiplist\b`), "跳表"},
		{regexp.MustCompile(`(?i)\blinked\s*list\b`), "链表"},
	}
}

// ApplySubjectTermAliases canonicalizes common CN/EN spellings of CS subjects.
func ApplySubjectTermAliases(q string) string {
	for _, item := range subjectTermAliasPatterns {
		q = item.re.ReplaceAllString(q, item.to)
	}
	return q
}

var subjectEnglishVariants = map[string][]string{
	"b+树":   {"b+tree", "b+ tree"},
	"红黑树":  {"rbtree", "red black tree"},
	"avl树":  {"avl tree", "avl"},
	"跳表":   {"skiplist", "skip list"},
	"链表":   {"linked list"},
}

// QuestionLookupVariants returns question strings to try for exact QA body lookup.
func QuestionLookupVariants(q string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(q)
	canon := PrepareQueryText(q)
	add(canon)
	for cn, variants := range subjectEnglishVariants {
		if !strings.Contains(canon, cn) {
			continue
		}
		for _, en := range variants {
			add(strings.Replace(canon, cn, en, 1))
		}
	}
	return out
}

// QuestionsEquivalent reports whether two questions should share exact QA cache.
func QuestionsEquivalent(q1, q2 string) bool {
	return PrepareQueryText(q1) == PrepareQueryText(q2)
}

// ExtractSubjects returns canonical topic entities mentioned in a question.
func ExtractSubjects(q string) []string {
	q = PrepareQueryText(q)
	found := map[string]struct{}{}
	masked := q
	for _, item := range subjectPatterns {
		for {
			loc := item.re.FindStringIndex(masked)
			if loc == nil {
				break
			}
			found[item.id] = struct{}{}
			masked = masked[:loc[0]] + strings.Repeat(" ", loc[1]-loc[0]) + masked[loc[1]:]
		}
	}
	out := make([]string, 0, len(found))
	for id := range found {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// SubjectsCompatible reports whether two questions may share semantic QA cache/RAG refs.
// When both mention concrete subjects, at least one canonical id must overlap.
func SubjectsCompatible(q1, q2 string) bool {
	s1 := ExtractSubjects(q1)
	s2 := ExtractSubjects(q2)
	if len(s1) == 0 || len(s2) == 0 {
		return true
	}
	for _, a := range s1 {
		for _, b := range s2 {
			if a == b {
				return true
			}
		}
	}
	return false
}
