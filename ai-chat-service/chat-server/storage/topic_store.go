package storage

import (
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type TopicMeta struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	QACount   int    `json:"qa_count"`
}

type ITopicStore interface {
	Get(ctx context.Context, topicID string) (*TopicMeta, error)
	List(ctx context.Context) ([]TopicMeta, error)
	Create(ctx context.Context, name string) (*TopicMeta, error)
	AttachQA(ctx context.Context, topicID, qaHash string) error
}

type topicStore struct{}

func NewTopicStore() ITopicStore {
	return &topicStore{}
}

func SummarizeTopicName(question string) string {
	q := strings.TrimSpace(question)
	q = strings.ReplaceAll(q, "\n", " ")
	if q == "" {
		return "未分类"
	}
	runes := []rune(q)
	if len(runes) > 48 {
		return string(runes[:48]) + "…"
	}
	return q
}

func MakeTopicID(name string) string {
	sum := sha256.Sum256([]byte(NormalizeQuestion(name)))
	return hex.EncodeToString(sum[:8])
}

func (s *topicStore) Get(ctx context.Context, topicID string) (*TopicMeta, error) {
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	raw, err := client.Get(ctx, kvstore.QATopicKey(topicID))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	meta := &TopicMeta{}
	if err := json.Unmarshal(raw, meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *topicStore) List(ctx context.Context) ([]TopicMeta, error) {
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	raw, err := client.Get(ctx, kvstore.QATopicIndexKey())
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, err
	}
	out := make([]TopicMeta, 0, len(ids))
	for _, id := range ids {
		meta, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if meta != nil {
			out = append(out, *meta)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].QACount > out[j].QACount
	})
	return out, nil
}

func (s *topicStore) Create(ctx context.Context, name string) (*TopicMeta, error) {
	name = SummarizeTopicName(name)
	id := MakeTopicID(name)
	if existing, err := s.Get(ctx, id); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	meta := &TopicMeta{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().Unix(),
		QACount:   0,
	}
	bytes, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	writePool := kvstore.GetWritePool()
	wc := writePool.Get()
	defer writePool.Put(wc)
	if err := wc.SetOrMod(ctx, kvstore.QATopicKey(id), string(bytes)); err != nil {
		return nil, err
	}
	if err := s.appendTopicToIndex(ctx, id); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *topicStore) AttachQA(ctx context.Context, topicID, qaHash string) error {
	if topicID == "" || qaHash == "" {
		return nil
	}
	readPool := kvstore.GetReadPool()
	rc := readPool.Get()
	defer readPool.Put(rc)

	var hashes []string
	raw, err := rc.Get(ctx, kvstore.QATopicQAKey(topicID))
	if err != nil {
		return err
	}
	if raw != nil {
		_ = json.Unmarshal(raw, &hashes)
	}
	for _, h := range hashes {
		if h == qaHash {
			return nil
		}
	}
	hashes = append(hashes, qaHash)
	indexBytes, err := json.Marshal(hashes)
	if err != nil {
		return err
	}

	meta, err := s.Get(ctx, topicID)
	if err != nil {
		return err
	}
	if meta == nil {
		return nil
	}
	meta.QACount = len(hashes)
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	writePool := kvstore.GetWritePool()
	wc := writePool.Get()
	defer writePool.Put(wc)
	if err := wc.SetOrMod(ctx, kvstore.QATopicQAKey(topicID), string(indexBytes)); err != nil {
		return err
	}
	return wc.SetOrMod(ctx, kvstore.QATopicKey(topicID), string(metaBytes))
}

func (s *topicStore) appendTopicToIndex(ctx context.Context, topicID string) error {
	readPool := kvstore.GetReadPool()
	rc := readPool.Get()
	defer readPool.Put(rc)
	var ids []string
	raw, err := rc.Get(ctx, kvstore.QATopicIndexKey())
	if err != nil {
		return err
	}
	if raw != nil {
		_ = json.Unmarshal(raw, &ids)
	}
	for _, id := range ids {
		if id == topicID {
			return nil
		}
	}
	ids = append(ids, topicID)
	indexBytes, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	writePool := kvstore.GetWritePool()
	wc := writePool.Get()
	defer writePool.Put(wc)
	return wc.SetOrMod(ctx, kvstore.QATopicIndexKey(), string(indexBytes))
}

// TopicNameValid 供测试或校验用。
func TopicNameValid(name string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(name)) > 0
}
