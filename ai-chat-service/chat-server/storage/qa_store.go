package storage

import (
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const maxQAKeyLen = 1024

type QAEntry struct {
	Hash   string
	Key    string
	Record *QACacheRecord
}

type IQAStore interface {
	GetByQuestion(ctx context.Context, question string) (*QACacheRecord, string, error)
	GetByHash(ctx context.Context, hash string) (*QACacheRecord, error)
	GetByKey(ctx context.Context, key string) (*QACacheRecord, error)
	SetQA(ctx context.Context, question string, record *QACacheRecord, ttlSec int) error
	ListAll(ctx context.Context) ([]QAEntry, error)
	RebuildIndex(ctx context.Context, hashes []string) error
}

type qaStore struct{}

func NewQAStore() IQAStore {
	return &qaStore{}
}

var spaceRe = regexp.MustCompile(`\s+`)

func NormalizeQuestion(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ToLower(q)
	q = spaceRe.ReplaceAllString(q, " ")
	return q
}

func QuestionHash(question string) string {
	h := sha256.Sum256([]byte(PrepareQueryText(question)))
	return hex.EncodeToString(h[:])
}

func metaToCacheRecord(meta *QAMetaRecord, answer string) *QACacheRecord {
	if meta == nil {
		return nil
	}
	return &QACacheRecord{
		Question:         meta.Question,
		Answer:           answer,
		Source:           meta.Source,
		TopicID:          meta.TopicID,
		TopicName:        meta.TopicName,
		PromptTokens:     meta.PromptTokens,
		CompletionTokens: meta.CompletionTokens,
		TotalTokens:      meta.TotalTokens,
		TokensSaved:      meta.TokensSaved,
		CreatedAt:        meta.CreatedAt,
	}
}

func (s *qaStore) getBodyAnswer(ctx context.Context, question string) (string, error) {
	answer, _, err := s.getBodyAnswerFromVariants(ctx, QuestionLookupVariants(question))
	return answer, err
}

func (s *qaStore) getBodyAnswerFromVariants(ctx context.Context, variants []string) (string, string, error) {
	for _, question := range variants {
		bodyKey := kvstore.QABodyKey(question)
		if len(bodyKey) > maxQAKeyLen {
			return "", "", fmt.Errorf("qa body key too long: %d bytes", len(bodyKey))
		}
		data, err := kvstore.GetWithReadFallback(ctx, bodyKey)
		if err != nil {
			return "", "", err
		}
		if data == nil {
			data, err = kvstore.GetWithReadFallback(ctx, kvstore.LegacyQABodyKey(question))
			if err != nil {
				return "", "", err
			}
		}
		if data != nil {
			return string(data), question, nil
		}
	}
	return "", "", nil
}

func (s *qaStore) getMeta(ctx context.Context, hash string) (*QAMetaRecord, error) {
	metaKey := kvstore.QAMetaKey(hash)
	data, err := kvstore.GetWithReadFallback(ctx, metaKey)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	meta := &QAMetaRecord{}
	if err := json.Unmarshal(data, meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *qaStore) GetByQuestion(ctx context.Context, question string) (*QACacheRecord, string, error) {
	variants := QuestionLookupVariants(question)
	bodyKey := kvstore.QABodyKey(PrepareQueryText(question))
	answer, matchedQuestion, err := s.getBodyAnswerFromVariants(ctx, variants)
	if err != nil {
		return nil, bodyKey, err
	}
	if answer == "" {
		return nil, bodyKey, nil
	}
	for _, variant := range variants {
		meta, err := s.getMeta(ctx, QuestionHash(variant))
		if err != nil {
			return nil, bodyKey, err
		}
		if meta != nil {
			return metaToCacheRecord(meta, answer), kvstore.QABodyKey(matchedQuestion), nil
		}
	}
	return &QACacheRecord{
		Question: question,
		Answer:   answer,
	}, kvstore.QABodyKey(matchedQuestion), nil
}

func (s *qaStore) GetByHash(ctx context.Context, hash string) (*QACacheRecord, error) {
	meta, err := s.getMeta(ctx, hash)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, nil
	}
	answer, err := s.getBodyAnswer(ctx, meta.Question)
	if err != nil {
		return nil, err
	}
	if answer == "" {
		return nil, nil
	}
	return metaToCacheRecord(meta, answer), nil
}

func (s *qaStore) GetByKey(ctx context.Context, key string) (*QACacheRecord, error) {
	prefix := kvstore.ServicePrefix()
	if strings.HasPrefix(key, prefix) {
		rest := strings.TrimPrefix(key, prefix)
		if strings.HasPrefix(rest, "qa_meta_") {
			hash := strings.TrimPrefix(rest, "qa_meta_")
			return s.GetByHash(ctx, hash)
		}
		if strings.HasPrefix(rest, "qa_body_") {
			question := strings.TrimPrefix(rest, "qa_body_")
			rec, _, err := s.GetByQuestion(ctx, question)
			return rec, err
		}
	}
	rec, _, err := s.GetByQuestion(ctx, key)
	return rec, err
}

func (s *qaStore) SetQA(ctx context.Context, question string, record *QACacheRecord, ttlSec int) error {
	if record == nil {
		return fmt.Errorf("nil QA record")
	}
	canonical := PrepareQueryText(question)
	bodyKey := kvstore.QABodyKey(canonical)
	if len(bodyKey) > maxQAKeyLen {
		return fmt.Errorf("qa body key too long: %d bytes", len(bodyKey))
	}
	hash := QuestionHash(question)
	meta := &QAMetaRecord{
		Question:         question,
		HashID:           hash,
		Source:           record.Source,
		TopicID:          record.TopicID,
		TopicName:        record.TopicName,
		PromptTokens:     record.PromptTokens,
		CompletionTokens: record.CompletionTokens,
		TotalTokens:      record.TotalTokens,
		TokensSaved:      record.TokensSaved,
		CreatedAt:        record.CreatedAt,
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	metaKey := kvstore.QAMetaKey(hash)

	writePool := kvstore.GetWritePool()
	client := writePool.Get()
	defer writePool.Put(client)

	if err := client.SetOrMod(ctx, bodyKey, record.Answer); err != nil {
		return err
	}
	for _, alias := range QuestionLookupVariants(question) {
		if alias == canonical {
			continue
		}
		if err := client.SetOrMod(ctx, kvstore.QABodyKey(alias), record.Answer); err != nil {
			return err
		}
	}
	if ttlSec <= 0 {
		if err := client.SetOrMod(ctx, metaKey, string(metaBytes)); err != nil {
			return err
		}
	} else if err := client.SetWithExpire(ctx, metaKey, string(metaBytes), ttlSec); err != nil {
		return err
	}
	return s.appendHashToIndex(ctx, hash)
}

func (s *qaStore) ListAll(ctx context.Context) ([]QAEntry, error) {
	hashes, err := s.loadIndexHashes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]QAEntry, 0, len(hashes))
	for _, hash := range hashes {
		key := kvstore.QAMetaKey(hash)
		rec, err := s.GetByHash(ctx, hash)
		if err != nil {
			return nil, err
		}
		if rec == nil {
			continue
		}
		out = append(out, QAEntry{
			Hash:   hash,
			Key:    key,
			Record: rec,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Record.CreatedAt > out[j].Record.CreatedAt
	})
	return out, nil
}

func (s *qaStore) RebuildIndex(ctx context.Context, hashes []string) error {
	seen := make(map[string]struct{}, len(hashes))
	unique := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		if hash == "" {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		unique = append(unique, hash)
	}
	indexBytes, err := json.Marshal(unique)
	if err != nil {
		return err
	}
	pool := kvstore.GetWritePool()
	client := pool.Get()
	defer pool.Put(client)
	return client.SetOrMod(ctx, kvstore.QAIndexKey(), string(indexBytes))
}

func (s *qaStore) loadIndexHashes(ctx context.Context) ([]string, error) {
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	raw, err := client.Get(ctx, kvstore.QAIndexKey())
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var hashes []string
	if err := json.Unmarshal(raw, &hashes); err != nil {
		return nil, err
	}
	return hashes, nil
}

func (s *qaStore) appendHashToIndex(ctx context.Context, hash string) error {
	readPool := kvstore.GetReadPool()
	rc := readPool.Get()
	defer readPool.Put(rc)
	var hashes []string
	raw, err := rc.Get(ctx, kvstore.QAIndexKey())
	if err != nil {
		return err
	}
	if raw != nil {
		_ = json.Unmarshal(raw, &hashes)
	}
	for _, h := range hashes {
		if h == hash {
			return nil
		}
	}
	hashes = append(hashes, hash)
	indexBytes, err := json.Marshal(hashes)
	if err != nil {
		return err
	}
	writePool := kvstore.GetWritePool()
	wc := writePool.Get()
	defer writePool.Put(wc)
	return wc.SetOrMod(ctx, kvstore.QAIndexKey(), string(indexBytes))
}
