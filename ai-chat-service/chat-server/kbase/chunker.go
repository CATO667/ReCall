package kbase

import (
	"strings"
	"unicode/utf8"
)

type Chunker interface {
	Split(text string) []ChunkDraft
}

func NewChunker(cfg ChunkConfig) Chunker {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 512
	}
	if cfg.ChunkOverlap < 0 {
		cfg.ChunkOverlap = 0
	}
	if cfg.ChunkOverlap >= cfg.ChunkSize {
		cfg.ChunkOverlap = cfg.ChunkSize / 5
	}
	switch strings.ToLower(cfg.Strategy) {
	case "fixed", "fixed_size":
		return &FixedSizeChunker{cfg: cfg}
	case "record", "line":
		return &RecordChunker{minSize: cfg.MinChunkSize}
	case "recursive":
		if len(cfg.Separators) == 0 {
			cfg.Separators = DefaultSeparators()
		}
		return &RecursiveChunker{cfg: cfg, minSize: cfg.MinChunkSize}
	case "novel":
		if len(cfg.Separators) == 0 {
			cfg.Separators = DefaultSeparators()
		}
		return &NovelChunker{
			inner:   RecursiveChunker{cfg: cfg},
			minSize: cfg.MinChunkSize,
		}
	default:
		if len(cfg.Separators) == 0 {
			cfg.Separators = DefaultSeparators()
		}
		return &NovelChunker{
			inner:   RecursiveChunker{cfg: cfg},
			minSize: cfg.MinChunkSize,
		}
	}
}

type FixedSizeChunker struct {
	cfg ChunkConfig
}

func (c *FixedSizeChunker) Split(text string) []ChunkDraft {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	size := c.cfg.ChunkSize
	overlap := c.cfg.ChunkOverlap
	step := size - overlap
	if step <= 0 {
		step = size
	}
	out := make([]ChunkDraft, 0, len(runes)/step+1)
	idx := 0
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		content := string(runes[start:end])
		if strings.TrimSpace(content) == "" {
			if end == len(runes) {
				break
			}
			continue
		}
		out = append(out, ChunkDraft{
			Index:    idx,
			Content:  content,
			StartPos: start,
			EndPos:   end,
		})
		idx++
		if end == len(runes) {
			break
		}
	}
	return out
}

// RecordChunker 按空行分段，每条记录独立成 chunk（适合投档表等结构化行数据）。
type RecordChunker struct {
	minSize int
}

func (c *RecordChunker) Split(text string) []ChunkDraft {
	parts := strings.Split(text, "\n\n")
	out := make([]ChunkDraft, 0, len(parts))
	pos := 0
	idx := 0
	var pendingChapter string
	for _, part := range parts {
		content := strings.TrimSpace(part)
		if content == "" {
			continue
		}
		if strings.HasPrefix(content, "#") {
			pendingChapter = strings.TrimSpace(strings.TrimLeft(content, "#"))
			continue
		}
		if c.minSize > 0 && utf8.RuneCountInString(content) < c.minSize {
			continue
		}
		endPos := pos + utf8.RuneCountInString(content)
		out = append(out, ChunkDraft{
			Index:    idx,
			Chapter:  pendingChapter,
			Content:  content,
			StartPos: pos,
			EndPos:   endPos,
		})
		idx++
		pos = endPos
	}
	return out
}

type RecursiveChunker struct {
	cfg     ChunkConfig
	minSize int
}

func (c *RecursiveChunker) Split(text string) []ChunkDraft {
	parts := c.splitRecursive(text, c.cfg.Separators)
	merged := c.mergeSmallParts(parts, c.cfg.ChunkSize)
	drafts := c.applyOverlap(merged)
	return filterDraftsByMinSize(drafts, c.minSize)
}

func (c *RecursiveChunker) splitRecursive(text string, separators []string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if utf8.RuneCountInString(text) <= c.cfg.ChunkSize {
		return []string{text}
	}
	if len(separators) == 0 {
		return splitByRunes(text, c.cfg.ChunkSize)
	}
	sep := separators[0]
	rest := separators[1:]
	if sep == "" {
		return splitByRunes(text, c.cfg.ChunkSize)
	}
	rawParts := strings.Split(text, sep)
	out := make([]string, 0, len(rawParts))
	for i, part := range rawParts {
		piece := part
		if i < len(rawParts)-1 && sep != "" {
			piece += sep
		}
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		if utf8.RuneCountInString(piece) <= c.cfg.ChunkSize {
			out = append(out, piece)
			continue
		}
		out = append(out, c.splitRecursive(piece, rest)...)
	}
	return out
}

func (c *RecursiveChunker) mergeSmallParts(parts []string, chunkSize int) []string {
	if len(parts) == 0 {
		return nil
	}
	merged := make([]string, 0, len(parts))
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		merged = append(merged, strings.TrimSpace(buf.String()))
		buf.Reset()
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if buf.Len() == 0 {
			buf.WriteString(part)
			if utf8.RuneCountInString(buf.String()) >= chunkSize {
				flush()
			}
			continue
		}
		candidate := buf.String() + "\n" + part
		if utf8.RuneCountInString(candidate) <= chunkSize {
			buf.Reset()
			buf.WriteString(candidate)
			continue
		}
		flush()
		buf.WriteString(part)
		if utf8.RuneCountInString(buf.String()) >= chunkSize {
			flush()
		}
	}
	flush()
	return merged
}

func (c *RecursiveChunker) applyOverlap(parts []string) []ChunkDraft {
	if len(parts) == 0 {
		return nil
	}
	out := make([]ChunkDraft, 0, len(parts))
	pos := 0
	for i, part := range parts {
		content := strings.TrimSpace(part)
		if content == "" {
			continue
		}
		if c.cfg.ChunkOverlap > 0 && i > 0 {
			prev := []rune(out[len(out)-1].Content)
			overlap := c.cfg.ChunkOverlap
			if overlap > len(prev) {
				overlap = len(prev)
			}
			prefix := string(prev[len(prev)-overlap:])
			content = strings.TrimSpace(prefix + content)
		}
		endPos := pos + utf8.RuneCountInString(content)
		out = append(out, ChunkDraft{
			Index:    len(out),
			Content:  content,
			StartPos: pos,
			EndPos:   endPos,
		})
		pos = endPos
	}
	return out
}

func splitByRunes(text string, chunkSize int) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	out := make([]string, 0, len(runes)/chunkSize+1)
	for start := 0; start < len(runes); start += chunkSize {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[start:end]))
	}
	return out
}
