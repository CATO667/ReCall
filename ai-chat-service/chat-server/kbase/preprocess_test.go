package kbase

import (
	"strings"
	"testing"
)

func TestPreprocessNovelRemovesAds(t *testing.T) {
	in := "==========\n搜书吧下载 http://www.soushu555.com\n==========\n\n《三千美娇娘》\n\n正文开始。"
	out := PreprocessNovel(in)
	if strings.Contains(out, "搜书吧") {
		t.Fatalf("ad not removed: %q", out)
	}
	if !strings.Contains(out, "正文开始") {
		t.Fatalf("content lost: %q", out)
	}
}

func TestSplitByChapter(t *testing.T) {
	text := "简介\n\n第一章 开端\n第一段。\n\n第二章 继续\n第二段。"
	sections := splitByChapter(text)
	if len(sections) < 2 {
		t.Fatalf("expected >=2 sections, got %d", len(sections))
	}
}

func TestSafeTruncateBytesGBK(t *testing.T) {
	raw := []byte{0xb8, 0xfc, 0xb6, 0xe0, 0xca, 0xc7, 0xb2, 0xe2, 0xca, 0xd4}
	truncated := safeTruncateBytes(append(raw, raw...), 5)
	content, err := decodeText(truncated)
	if err != nil {
		t.Fatal(err)
	}
	if content == "" {
		t.Fatal("expected decoded content")
	}
}
