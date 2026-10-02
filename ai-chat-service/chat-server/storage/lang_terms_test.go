package storage

import "testing"

func TestApplyLanguageAliases(t *testing.T) {
	got := ApplyLanguageAliases(NormalizeQuestion("用 Golang 和 CPP 写代码"))
	want := "用 go 和 c++ 写代码"
	if got != want {
		t.Fatalf("ApplyLanguageAliases() = %q, want %q", got, want)
	}
}

func TestExtractLanguages(t *testing.T) {
	cases := []struct {
		q    string
		want []string
	}{
		{"用C语言生成一颗红黑树", []string{"c"}},
		{"用go语言生成链表", []string{"go"}},
		{"用c++实现vector", []string{"c++"}},
		{"用cpp实现vector", []string{"c++"}},
		{"什么是链表", nil},
		{"implement a linked list in c", []string{"c"}},
		{"implement in c++", []string{"c++"}},
	}
	for _, tc := range cases {
		got := ExtractLanguages(tc.q)
		if len(got) != len(tc.want) {
			t.Fatalf("ExtractLanguages(%q) = %v, want %v", tc.q, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ExtractLanguages(%q) = %v, want %v", tc.q, got, tc.want)
			}
		}
	}
}

func TestLanguagesCompatible(t *testing.T) {
	cases := []struct {
		q1, q2 string
		want   bool
	}{
		{"用go语言生成链表", "用c语言生成链表", false},
		{"用c语言生成一颗红黑树", "用c语言生成一颗rbtree", true},
		{"生成链表", "用c语言生成链表", true},
		{"用c++写代码", "用c写代码", false},
	}
	for _, tc := range cases {
		got := LanguagesCompatible(tc.q1, tc.q2)
		if got != tc.want {
			t.Fatalf("LanguagesCompatible(%q, %q) = %v, want %v", tc.q1, tc.q2, got, tc.want)
		}
	}
}

func TestPrepareQueryText(t *testing.T) {
	got := PrepareQueryText("  用 Golang 写  RBtree  ")
	want := "用 go 写 红黑树"
	if got != want {
		t.Fatalf("PrepareQueryText() = %q, want %q", got, want)
	}
}
