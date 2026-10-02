package server

import (
	"ai-chat-service/chat-server/storage"
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"strings"
)

type TopicClassifier struct {
	qaStore     storage.IQAStore
	topicStore  storage.ITopicStore
	vectorStore vector.VectorStore
	threshold   float32
}

func NewTopicClassifier(qaStore storage.IQAStore, topicStore storage.ITopicStore, vectorStore vector.VectorStore, cnf *config.Config) *TopicClassifier {
	threshold := float32(0.82)
	if cnf != nil && cnf.Vector.TopicSimilarityThreshold > 0 {
		threshold = cnf.Vector.TopicSimilarityThreshold
	}
	return &TopicClassifier{
		qaStore:     qaStore,
		topicStore:  topicStore,
		vectorStore: vectorStore,
		threshold:   threshold,
	}
}

// AssignTopic 根据向量相似度将 QA 归入已有主题，或创建新主题。
func (c *TopicClassifier) AssignTopic(ctx context.Context, question, qaHash string, embedding []float32) (topicID, topicName string, err error) {
	if c == nil || c.topicStore == nil {
		return "", "", nil
	}

	if c.vectorStore != nil && len(embedding) > 0 {
		normalized := prepareQueryText(question)
		hits, searchErr := c.vectorStore.SearchTopK(ctx, normalized, embedding, 5, vector.RecallScoreFilter)
		if searchErr == nil {
			for _, hit := range hits {
				if hit.Score < c.threshold {
					continue
				}
				hitHash := qaHashFromKey(hit.QAKey)
				if hitHash == qaHash {
					continue
				}
				rec, getErr := c.qaStore.GetByHash(ctx, qaHashFromKey(hit.QAKey))
				if getErr != nil || rec == nil {
					continue
				}
				if rec.TopicID != "" {
					_ = c.topicStore.AttachQA(ctx, rec.TopicID, qaHash)
					return rec.TopicID, rec.TopicName, nil
				}
				// 相似 QA 尚未分类：以该问题为代表创建主题，并归入当前 QA
				meta, createErr := c.topicStore.Create(ctx, rec.Question)
				if createErr != nil {
					continue
				}
				_ = c.topicStore.AttachQA(ctx, meta.ID, hitHash)
				_ = c.topicStore.AttachQA(ctx, meta.ID, qaHash)
				return meta.ID, meta.Name, nil
			}
		}
	}

	meta, err := c.topicStore.Create(ctx, question)
	if err != nil {
		return "", "", err
	}
	_ = c.topicStore.AttachQA(ctx, meta.ID, qaHash)
	return meta.ID, meta.Name, nil
}

func qaHashFromKey(qaKey string) string {
	prefix := kvstore.ServicePrefix()
	if !strings.HasPrefix(qaKey, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(qaKey, prefix)
	if strings.HasPrefix(rest, "qa_meta_") {
		return strings.TrimPrefix(rest, "qa_meta_")
	}
	if strings.HasPrefix(rest, "qa_") && !strings.HasPrefix(rest, "qa_body_") &&
		rest != "qa_index" && !strings.HasPrefix(rest, "qa_topic_") {
		return strings.TrimPrefix(rest, "qa_")
	}
	return ""
}
