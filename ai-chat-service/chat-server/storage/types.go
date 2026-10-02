package storage

type QAMetaRecord struct {
	Question         string `json:"question"`
	HashID           string `json:"hash_id"`
	Source           string `json:"source"`
	TopicID          string `json:"topic_id,omitempty"`
	TopicName        string `json:"topic_name,omitempty"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	TokensSaved      int    `json:"tokens_saved"`
	CreatedAt        int64  `json:"created_at"`
}

type QACacheRecord struct {
	Question         string  `json:"question"`
	Answer           string  `json:"answer"`
	Source           string  `json:"source"`
	TopicID          string  `json:"topic_id,omitempty"`
	TopicName        string  `json:"topic_name,omitempty"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TokensSaved      int     `json:"tokens_saved"`
	CreatedAt        int64   `json:"created_at"`
	SimilarityScore  float32 `json:"similarity_score,omitempty"`
}

const (
	SourceLLM       = "llm"
	SourceCache     = "cache"
	SourceSensitive = "sensitive"
	SourceRAG       = "rag"
)
