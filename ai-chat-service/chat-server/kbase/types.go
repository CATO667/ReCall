package kbase

import "time"

const (
	SourceTypeText     = "txt"
	SourceTypeMarkdown = "md"
)

type DocumentMeta struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	SourcePath  string `json:"source_path,omitempty"`
	SourceType  string `json:"source_type"`
	ContentHash string `json:"content_hash"`
	ChunkCount  int    `json:"chunk_count"`
	CharCount   int    `json:"char_count"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type Chunk struct {
	ID       string `json:"id"`
	DocID    string `json:"doc_id"`
	Index    int    `json:"index"`
	Chapter  string `json:"chapter,omitempty"`
	Content  string `json:"content"`
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
}

type ChunkDraft struct {
	Index    int
	Chapter  string
	Content  string
	StartPos int
	EndPos   int
}

type ParsedDocument struct {
	Title   string
	Content string
	Type    string
}

type IngestInput struct {
	Title      string
	Content    string
	SourcePath string
	SourceType string
}

type IngestResult struct {
	Document *DocumentMeta
	Chunks   []Chunk
	Indexed  int
	Skipped  bool
}

type ChunkConfig struct {
	Strategy         string
	ChunkSize        int
	ChunkOverlap     int
	MinChunkSize     int
	Separators       []string
}

func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		Strategy:     "novel",
		ChunkSize:    512,
		ChunkOverlap: 64,
		MinChunkSize: 50,
		Separators:   DefaultSeparators(),
	}
}

func DefaultSeparators() []string {
	return []string{"\n\n", "\n", "。", "！", "？", ".", "!", "?", "；", ";", " ", ""}
}

func nowUnix() int64 {
	return time.Now().Unix()
}
