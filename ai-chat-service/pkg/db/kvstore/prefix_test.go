package kvstore

import "testing"

func TestParseQAHashFromKey(t *testing.T) {
	hash := "8e0ab75b5ab842ad0181bc587ce34c014cd18cb5f8b70712d51e510c704d3334"
	tests := []struct {
		key  string
		want string
	}{
		{"ai_chat_service_qa_meta_" + hash, hash},
		{"ai_chat_service_qa_" + hash, hash},
		{"ai_chat_service_qa_body_question", ""},
		{"ai_chat_service_qa_index", ""},
		{"other_prefix_qa_meta_" + hash, ""},
	}
	for _, tc := range tests {
		if got := ParseQAHashFromKey(tc.key); got != tc.want {
			t.Fatalf("ParseQAHashFromKey(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}
