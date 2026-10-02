package kbase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Parser interface {
	ParseFile(path string) (*ParsedDocument, error)
	ParseFileWithLimit(path string, maxBytes int) (*ParsedDocument, error)
	ParseBytes(name string, raw []byte) (*ParsedDocument, error)
}

type textParser struct{}

func NewParser() Parser {
	return &textParser{}
}

func (p *textParser) ParseFile(path string) (*ParsedDocument, error) {
	return p.ParseFileWithLimit(path, 0)
}

func (p *textParser) ParseFileWithLimit(path string, maxBytes int) (*ParsedDocument, error) {
	var content string
	var err error
	if maxBytes > 0 {
		content, err = ReadFileDecoded(path, maxBytes)
	} else {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		content, err = decodeText(raw)
	}
	if err != nil {
		return nil, err
	}
	return p.buildParsed(path, content)
}

func (p *textParser) ParseBytes(name string, raw []byte) (*ParsedDocument, error) {
	content, err := decodeText(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return p.buildParsed(name, content)
}

func (p *textParser) buildParsed(name, content string) (*ParsedDocument, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty document: %s", name)
	}
	sourceType := detectSourceType(name)
	content = PreprocessContent(content, sourceType)
	title := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if sourceType == SourceTypeMarkdown {
		title = extractMarkdownTitle(content, title)
	}
	return &ParsedDocument{
		Title:   title,
		Content: content,
		Type:    sourceType,
	}, nil
}

func detectSourceType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown":
		return SourceTypeMarkdown
	default:
		return SourceTypeText
	}
}

func extractMarkdownTitle(content, fallback string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}
