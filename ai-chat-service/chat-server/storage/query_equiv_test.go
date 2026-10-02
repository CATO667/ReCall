package storage

import "testing"

func TestPrepareQueryText_BPlusTreeAliases(t *testing.T) {
	got := PrepareQueryText("  C语言生成 B+tree  ")
	want := "c语言生成 b+树"
	if got != want {
		t.Fatalf("PrepareQueryText() = %q, want %q", got, want)
	}
}

func TestQuestionHash_EquivalentForms(t *testing.T) {
	h1 := QuestionHash("c语言生成b+树")
	h2 := QuestionHash("c语言生成b+tree")
	if h1 != h2 {
		t.Fatalf("QuestionHash mismatch: %q vs %q", h1, h2)
	}
}

func TestQuestionLookupVariants_BPlusTree(t *testing.T) {
	variants := QuestionLookupVariants("c语言生成b+树")
	hasTree := false
	hasCN := false
	for _, v := range variants {
		if v == "c语言生成b+tree" {
			hasTree = true
		}
		if v == "c语言生成b+树" {
			hasCN = true
		}
	}
	if !hasTree || !hasCN {
		t.Fatalf("QuestionLookupVariants() = %v, want both CN and EN forms", variants)
	}
}

func TestQuestionsEquivalent(t *testing.T) {
	if !QuestionsEquivalent("c语言生成b+树", "c语言生成b+tree") {
		t.Fatal("expected b+树 and b+tree to be equivalent")
	}
}
