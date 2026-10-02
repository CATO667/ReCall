package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type QAExportItem struct {
	Hash      string         `json:"hash"`
	Key       string         `json:"key"`
	Question  string         `json:"question"`
	Answer    string         `json:"answer"`
	Source    string         `json:"source"`
	TopicID   string         `json:"topic_id,omitempty"`
	TopicName string         `json:"topic_name,omitempty"`
	CreatedAt int64          `json:"created_at"`
	Record    *QACacheRecord `json:"record,omitempty"`
}

func EntriesToExportItems(entries []QAEntry) []QAExportItem {
	out := make([]QAExportItem, 0, len(entries))
	for _, e := range entries {
		if e.Record == nil {
			continue
		}
		out = append(out, QAExportItem{
			Hash:      e.Hash,
			Key:       e.Key,
			Question:  e.Record.Question,
			Answer:    e.Record.Answer,
			Source:    e.Record.Source,
			TopicID:   e.Record.TopicID,
			TopicName: e.Record.TopicName,
			CreatedAt: e.Record.CreatedAt,
			Record:    e.Record,
		})
	}
	return out
}

func ExportJSON(items []QAExportItem) ([]byte, error) {
	return json.MarshalIndent(items, "", "  ")
}

func ExportMarkdown(items []QAExportItem) string {
	var b strings.Builder
	b.WriteString("# 技术问答导出\n\n")
	b.WriteString(fmt.Sprintf("> 共 %d 条，导出时间 %s\n\n", len(items), time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("---\n\n")
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		created := "-"
		if item.CreatedAt > 0 {
			created = time.Unix(item.CreatedAt, 0).Format("2006-01-02 15:04:05")
		}
		source := item.Source
		if source == "" {
			source = "-"
		}
		b.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, item.Question))
		b.WriteString(fmt.Sprintf("- 时间: %s\n", created))
		b.WriteString(fmt.Sprintf("- 来源: %s\n\n", source))
		b.WriteString("### 回答\n\n")
		b.WriteString(item.Answer)
		b.WriteString("\n")
	}
	return b.String()
}

func ExportMarkdownByTopic(items []QAExportItem) string {
	groups := map[string][]QAExportItem{}
	order := make([]string, 0)
	for _, item := range items {
		key := item.TopicName
		if key == "" {
			key = "未分类"
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], item)
	}
	sort.Strings(order)
	// 未分类放最后
	for i, name := range order {
		if name == "未分类" {
			order = append(order[:i], order[i+1:]...)
			order = append(order, "未分类")
			break
		}
	}

	var b strings.Builder
	b.WriteString("# 技术问答导出（按主题）\n\n")
	b.WriteString(fmt.Sprintf("> 共 %d 条，%d 个主题，导出时间 %s\n\n", len(items), len(order), time.Now().Format("2006-01-02 15:04:05")))

	for gi, topicName := range order {
		group := groups[topicName]
		if gi > 0 {
			b.WriteString("\n---\n\n")
		}
		b.WriteString(fmt.Sprintf("# %s\n\n", topicName))
		b.WriteString(fmt.Sprintf("> 本主题 %d 条\n\n", len(group)))
		for i, item := range group {
			if i > 0 {
				b.WriteString("\n---\n\n")
			}
			created := "-"
			if item.CreatedAt > 0 {
				created = time.Unix(item.CreatedAt, 0).Format("2006-01-02 15:04:05")
			}
			b.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, item.Question))
			b.WriteString(fmt.Sprintf("- 时间: %s\n\n", created))
			b.WriteString("### 回答\n\n")
			b.WriteString(item.Answer)
			b.WriteString("\n")
		}
	}
	return b.String()
}
