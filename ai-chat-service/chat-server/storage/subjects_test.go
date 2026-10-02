package storage

import "testing"

func TestExtractSubjects(t *testing.T) {
	cases := []struct {
		q    string
		want []string
	}{
		{"c语言生成b+树", []string{"b+树"}},
		{"c语言生成b+tree", []string{"b+树"}},
		{"c语言生成红黑树", []string{"红黑树"}},
		{"用c语言生成一颗红黑树", []string{"红黑树"}},
		{"用c语言生成跳表", []string{"跳表"}},
		{"什么是快速排序", nil},
	}
	for _, tc := range cases {
		got := ExtractSubjects(tc.q)
		if len(got) != len(tc.want) {
			t.Fatalf("ExtractSubjects(%q) = %v, want %v", tc.q, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ExtractSubjects(%q) = %v, want %v", tc.q, got, tc.want)
			}
		}
	}
}

func TestSemanticCacheGate(t *testing.T) {
	paraphraseQ := "生成一颗红黑树，c语言描述"
	storedQ := "用c语言生成一颗红黑树"
	if !SubjectsCompatible(paraphraseQ, storedQ) {
		t.Fatal("paraphrase rbtree questions should be subject-compatible")
	}
	if SubjectsCompatible("c语言生成b+树", "c语言生成红黑树") {
		t.Fatal("b+ tree vs rbtree should not share cache")
	}
}

func TestSubjectsCompatible(t *testing.T) {
	cases := []struct {
		q1, q2 string
		want   bool
	}{
		{"c语言生成b+树", "c语言生成红黑树", false},
		{"c语言生成b+树", "用c语言生成跳表", false},
		{"c语言生成红黑树", "用c语言生成一颗红黑树", true},
		{"生成一棵树", "c语言生成红黑树", true},
		{"c语言生成跳表", "用c语言生成跳表", true},
		{"c语言生成b+树", "c语言生成b+tree", true},
	}
	for _, tc := range cases {
		got := SubjectsCompatible(tc.q1, tc.q2)
		if got != tc.want {
			t.Fatalf("SubjectsCompatible(%q, %q) = %v, want %v", tc.q1, tc.q2, got, tc.want)
		}
	}
}
