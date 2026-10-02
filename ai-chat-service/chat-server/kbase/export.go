package kbase

import (
	"fmt"
	"strings"
	"time"
)

func ExportDocumentMarkdown(meta *DocumentMeta, chunks []Chunk) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", meta.Title))
	if meta.SourcePath != "" {
		b.WriteString(fmt.Sprintf("> 源文件: %s\n", meta.SourcePath))
	}
	b.WriteString(fmt.Sprintf("> doc_id: %s | chunks: %d | chars: %d\n\n", meta.ID, meta.ChunkCount, meta.CharCount))
	if meta.UpdatedAt > 0 {
		b.WriteString(fmt.Sprintf("> 更新时间: %s\n\n", time.Unix(meta.UpdatedAt, 0).Format("2006-01-02 15:04:05")))
	}
	b.WriteString("---\n\n")
	for i, ch := range chunks {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		if ch.Chapter != "" {
			b.WriteString(fmt.Sprintf("## %s\n\n", ch.Chapter))
		} else if len(chunks) > 1 {
			b.WriteString(fmt.Sprintf("## 片段 %d\n\n", ch.Index+1))
		}
		b.WriteString(ch.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func ExportAllDocumentsMarkdown(metas []DocumentMeta, loader func(docID string) ([]Chunk, error)) (string, error) {
	var b strings.Builder
	b.WriteString("# 知识库文档导出\n\n")
	b.WriteString(fmt.Sprintf("> 共 %d 篇，导出时间 %s\n\n", len(metas), time.Now().Format("2006-01-02 15:04:05")))
	for i, meta := range metas {
		chunks, err := loader(meta.ID)
		if err != nil {
			return "", err
		}
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		b.WriteString(ExportDocumentMarkdown(&meta, chunks))
	}
	return b.String(), nil
}
