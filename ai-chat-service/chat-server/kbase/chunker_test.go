package kbase

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFixedSizeChunkerSplit(t *testing.T) {
	cfg := ChunkConfig{Strategy: "fixed", ChunkSize: 10, ChunkOverlap: 2}
	c := NewChunker(cfg)
	text := strings.Repeat("你好", 20)
	chunks := c.Split(text)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, ch := range chunks {
		if utf8.RuneCountInString(ch.Content) > cfg.ChunkSize {
			t.Fatalf("chunk too large: %d > %d", utf8.RuneCountInString(ch.Content), cfg.ChunkSize)
		}
	}
}

func TestRecursiveChunkerPrefersParagraphs(t *testing.T) {
	cfg := ChunkConfig{Strategy: "recursive", ChunkSize: 40, ChunkOverlap: 0}
	c := NewChunker(cfg)
	text := "第一段内容比较长，用于测试段落切分逻辑。\n\n第二段也是独立段落，应该尽量单独成块。\n\n第三段收尾。"
	chunks := c.Split(text)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	if !strings.Contains(chunks[0].Content, "第一段") {
		t.Fatalf("unexpected first chunk: %q", chunks[0].Content)
	}
}

func TestRecursiveChunkerOverlap(t *testing.T) {
	cfg := ChunkConfig{Strategy: "recursive", ChunkSize: 20, ChunkOverlap: 5}
	c := NewChunker(cfg)
	text := strings.Repeat("这是测试段落内容。", 8) + "\n\n" + strings.Repeat("第二段内容继续。", 8)
	chunks := c.Split(text)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(chunks))
	}
}
