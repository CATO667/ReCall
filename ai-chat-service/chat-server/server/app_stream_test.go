package server

import (
	"ai-chat-service/proto"
	"strings"
	"testing"
)

func testStreamApp() *app {
	return &app{openaiConf: &openaiConf{Model: "test-model"}}
}

func TestBuildChatCompletionStreamResponseList_CacheBulk(t *testing.T) {
	app := testStreamApp()
	msg := strings.Repeat("x", 10000)
	list := app.buildChatCompletionStreamResponseList("id", msg, proto.ResponseSource_SOURCE_CACHE, nil, 42)
	if len(list) != 3 {
		t.Fatalf("cache stream chunks = %d, want 3 (start + body + stop)", len(list))
	}
	if got := list[1].Choices[0].Delta.Content; got != msg {
		t.Fatalf("cache body chunk len = %d, want %d", len(got), len(msg))
	}
	if list[2].TokensSaved != 42 {
		t.Fatalf("tokensSaved = %d, want 42", list[2].TokensSaved)
	}
}

func TestBuildChatCompletionStreamResponseList_SensitiveBulk(t *testing.T) {
	app := testStreamApp()
	list := app.buildChatCompletionStreamResponseList("id", "触发到了知识盲区", proto.ResponseSource_SOURCE_SENSITIVE, nil, 0)
	if len(list) != 3 {
		t.Fatalf("sensitive stream chunks = %d, want 3", len(list))
	}
}

func TestBuildChatCompletionStreamResponseList_LLMPerRune(t *testing.T) {
	app := testStreamApp()
	msg := "abc"
	list := app.buildChatCompletionStreamResponseList("id", msg, proto.ResponseSource_SOURCE_LLM, nil, 0)
	if len(list) != len(msg)+2 {
		t.Fatalf("llm stream chunks = %d, want %d", len(list), len(msg)+2)
	}
}
