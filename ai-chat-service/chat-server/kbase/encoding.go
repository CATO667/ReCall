package kbase

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func decodeText(raw []byte) (string, error) {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if len(raw) == 0 {
		return "", fmt.Errorf("empty content")
	}
	if utf8.Valid(raw) {
		return normalizeNewlines(string(raw)), nil
	}
	decoded, err := decodeWithGB18030(raw)
	if err != nil {
		return "", fmt.Errorf("unsupported text encoding: %w", err)
	}
	return normalizeNewlines(decoded), nil
}

func decodeWithGB18030(raw []byte) (string, error) {
	reader := transform.NewReader(bytes.NewReader(raw), simplifiedchinese.GB18030.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(decoded) {
		return "", fmt.Errorf("gb18030 decode produced invalid utf-8")
	}
	return string(decoded), nil
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
