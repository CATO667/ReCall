package kbase

import (
	"fmt"
	"os"
	"path/filepath"
)

func ReadFileDecoded(path string, maxBytes int) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if maxBytes > 0 && len(raw) > maxBytes {
		raw = safeTruncateBytes(raw, maxBytes)
	}
	content, err := decodeText(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return content, nil
}

func safeTruncateBytes(raw []byte, maxBytes int) []byte {
	if maxBytes <= 0 || len(raw) <= maxBytes {
		return raw
	}
	truncated := raw[:maxBytes]
	for len(truncated) > 0 {
		if _, err := decodeText(truncated); err == nil {
			return truncated
		}
		truncated = truncated[:len(truncated)-1]
	}
	return raw[:maxBytes]
}

func SaveUTF8Sample(srcPath, outPath string, maxBytes int) (string, error) {
	content, err := ReadFileDecoded(srcPath, maxBytes)
	if err != nil {
		return "", err
	}
	content = PreprocessContent(content, detectSourceType(srcPath))
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return content, nil
}
