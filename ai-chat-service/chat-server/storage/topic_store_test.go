package storage

import (
	"strings"
	"testing"
)

func TestSummarizeTopicName(t *testing.T) {
	name := SummarizeTopicName("  C++ 智能指针 shared_ptr 用法？  ")
	if !strings.Contains(name, "智能指针") {
		t.Fatalf("unexpected name: %q", name)
	}
	long := SummarizeTopicName(strings.Repeat("问", 60))
	if len([]rune(long)) > 50 {
		t.Fatalf("name should be truncated: %q", long)
	}
}

func TestMakeTopicIDStable(t *testing.T) {
	a := MakeTopicID("C++ 智能指针")
	b := MakeTopicID("c++  智能指针")
	if a != b {
		t.Fatalf("topic id should be stable after normalize: %s vs %s", a, b)
	}
}

func TestExportMarkdownByTopic(t *testing.T) {
	items := []QAExportItem{
		{Question: "q1", Answer: "a1", TopicName: "C++"},
		{Question: "q2", Answer: "a2", TopicName: "C++"},
		{Question: "q3", Answer: "a3", TopicName: "Go"},
	}
	md := ExportMarkdownByTopic(items)
	if !strings.Contains(md, "# C++") || !strings.Contains(md, "# Go") {
		t.Fatalf("missing topic headers: %s", md)
	}
}
