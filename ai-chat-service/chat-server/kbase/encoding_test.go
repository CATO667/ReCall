package kbase

import (
	"testing"
)

func TestDecodeTextUTF8(t *testing.T) {
	text, err := decodeText([]byte("你好，世界"))
	if err != nil {
		t.Fatal(err)
	}
	if text != "你好，世界" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestDecodeTextGB18030(t *testing.T) {
	// "更多" in GB18030
	raw := []byte{0xb8, 0xfc, 0xb6, 0xe0}
	text, err := decodeText(raw)
	if err != nil {
		t.Fatal(err)
	}
	if text != "更多" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestDecodeTextUTF8BOM(t *testing.T) {
	raw := append([]byte{0xEF, 0xBB, 0xBF}, []byte("测试")...)
	text, err := decodeText(raw)
	if err != nil {
		t.Fatal(err)
	}
	if text != "测试" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestNormalizeNewlines(t *testing.T) {
	got := normalizeNewlines("a\r\nb\rc\r")
	want := "a\nb\nc\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
