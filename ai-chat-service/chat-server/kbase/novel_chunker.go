package kbase

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var chapterTitleRe = regexp.MustCompile(`^第[0-9一二三四五六七八九十百千零两]+章[^\n]{0,40}`)

type chapterSection struct {
	title string
	body  string
}

type NovelChunker struct {
	inner   RecursiveChunker
	minSize int
}

func (c *NovelChunker) Split(text string) []ChunkDraft {
	sections := splitByChapter(text)
	out := make([]ChunkDraft, 0)
	idx := 0
	for _, sec := range sections {
		drafts := c.inner.Split(sec.body)
		for _, d := range drafts {
			if c.minSize > 0 && utf8.RuneCountInString(d.Content) < c.minSize {
				continue
			}
			d.Index = idx
			d.Chapter = sec.title
			out = append(out, d)
			idx++
		}
	}
	return out
}

func splitByChapter(text string) []chapterSection {
	lines := strings.Split(text, "\n")
	sections := make([]chapterSection, 0)
	var current chapterSection
	hasChapter := false

	flush := func() {
		body := strings.TrimSpace(current.body)
		if body == "" {
			return
		}
		title := current.title
		if title == "" {
			title = "前言"
		}
		sections = append(sections, chapterSection{title: title, body: body})
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && chapterTitleRe.MatchString(trimmed) {
			hasChapter = true
			flush()
			current = chapterSection{title: trimmed, body: ""}
			continue
		}
		if current.body == "" && current.title == "" {
			current.body = line
		} else if current.body == "" {
			current.body = line
		} else {
			current.body += "\n" + line
		}
	}
	flush()

	if !hasChapter || len(sections) == 0 {
		return []chapterSection{{title: "全文", body: text}}
	}
	return sections
}

func filterDraftsByMinSize(drafts []ChunkDraft, minSize int) []ChunkDraft {
	if minSize <= 0 {
		return drafts
	}
	out := make([]ChunkDraft, 0, len(drafts))
	idx := 0
	for _, d := range drafts {
		if utf8.RuneCountInString(d.Content) < minSize {
			continue
		}
		d.Index = idx
		out = append(out, d)
		idx++
	}
	return out
}
