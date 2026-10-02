package storage

import (
	"strings"
	"testing"
)

func TestExportMarkdown(t *testing.T) {
	items := []QAExportItem{
		{
			Hash:      "abc",
			Question:  "什么是智能指针？",
			Answer:    "智能指针是 RAII 的封装。",
			Source:    "llm",
			CreatedAt: 1720000000,
		},
	}
	md := ExportMarkdown(items)
	if !strings.Contains(md, "什么是智能指针？") {
		t.Fatalf("missing question: %s", md)
	}
	if !strings.Contains(md, "智能指针是 RAII 的封装。") {
		t.Fatalf("missing answer: %s", md)
	}
	if !strings.Contains(md, "共 1 条") {
		t.Fatalf("missing count: %s", md)
	}
}

func TestExportJSON(t *testing.T) {
	items := []QAExportItem{{Hash: "abc", Question: "q", Answer: "a"}}
	data, err := ExportJSON(items)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"question": "q"`) {
		t.Fatalf("unexpected json: %s", string(data))
	}
}
