package kbase

import (
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type DocumentStore interface {
	GetMeta(ctx context.Context, docID string) (*DocumentMeta, error)
	ListMeta(ctx context.Context) ([]DocumentMeta, error)
	GetChunks(ctx context.Context, docID string) ([]Chunk, error)
	Save(ctx context.Context, doc *DocumentMeta, chunks []Chunk) error
	Delete(ctx context.Context, docID string) error
	FindByContentHash(ctx context.Context, hash string) (*DocumentMeta, error)
}

type kvDocumentStore struct{}

func NewDocumentStore() DocumentStore {
	return &kvDocumentStore{}
}

func docContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func makeDocID(sourcePath, contentHash string) string {
	base := sourcePath
	if base == "" {
		base = "inline"
	}
	sum := sha256.Sum256([]byte(base + ":" + contentHash))
	return hex.EncodeToString(sum[:16])
}

func makeChunkID(docID string, index int) string {
	return fmt.Sprintf("%s_%04d", docID, index)
}

func (s *kvDocumentStore) GetMeta(ctx context.Context, docID string) (*DocumentMeta, error) {
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	raw, err := client.Get(ctx, kvstore.DocMetaKey(docID))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	meta := &DocumentMeta{}
	if err := json.Unmarshal(raw, meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *kvDocumentStore) ListMeta(ctx context.Context) ([]DocumentMeta, error) {
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	raw, err := client.Get(ctx, kvstore.DocIndexKey())
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
	out := make([]DocumentMeta, 0, len(ids))
	for _, id := range ids {
		meta, err := s.GetMeta(ctx, id)
		if err != nil {
			return nil, err
		}
		if meta != nil {
			out = append(out, *meta)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

func (s *kvDocumentStore) GetChunks(ctx context.Context, docID string) ([]Chunk, error) {
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	raw, err := client.Get(ctx, kvstore.DocChunkIndexKey(docID))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var chunkIDs []string
	if err := json.Unmarshal(raw, &chunkIDs); err != nil {
		return nil, err
	}
	out := make([]Chunk, 0, len(chunkIDs))
	for _, chunkID := range chunkIDs {
		data, err := client.Get(ctx, kvstore.ChunkKey(chunkID))
		if err != nil {
			return nil, err
		}
		if data == nil {
			continue
		}
		ch := Chunk{}
		if err := json.Unmarshal(data, &ch); err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Index < out[j].Index
	})
	return out, nil
}

func (s *kvDocumentStore) Save(ctx context.Context, doc *DocumentMeta, chunks []Chunk) error {
	writePool := kvstore.GetWritePool()
	wc := writePool.Get()
	defer writePool.Put(wc)

	metaBytes, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if err := wc.SetOrMod(ctx, kvstore.DocMetaKey(doc.ID), string(metaBytes)); err != nil {
		return err
	}
	if doc.ContentHash != "" {
		if err := wc.SetOrMod(ctx, kvstore.DocHashKey(doc.ContentHash), doc.ID); err != nil {
			return err
		}
	}

	chunkIDs := make([]string, 0, len(chunks))
	for _, ch := range chunks {
		chBytes, err := json.Marshal(ch)
		if err != nil {
			return err
		}
		if err := wc.SetOrMod(ctx, kvstore.ChunkKey(ch.ID), string(chBytes)); err != nil {
			return err
		}
		chunkIDs = append(chunkIDs, ch.ID)
	}
	chunkIndexBytes, err := json.Marshal(chunkIDs)
	if err != nil {
		return err
	}
	if err := wc.SetOrMod(ctx, kvstore.DocChunkIndexKey(doc.ID), string(chunkIndexBytes)); err != nil {
		return err
	}

	readPool := kvstore.GetReadPool()
	rc := readPool.Get()
	defer readPool.Put(rc)
	var ids []string
	raw, err := rc.Get(ctx, kvstore.DocIndexKey())
	if err != nil {
		return err
	}
	if raw != nil {
		_ = json.Unmarshal(raw, &ids)
	}
	found := false
	for _, id := range ids {
		if id == doc.ID {
			found = true
			break
		}
	}
	if !found {
		ids = append(ids, doc.ID)
		indexBytes, err := json.Marshal(ids)
		if err != nil {
			return err
		}
		return wc.SetOrMod(ctx, kvstore.DocIndexKey(), string(indexBytes))
	}
	return nil
}

func (s *kvDocumentStore) Delete(ctx context.Context, docID string) error {
	meta, _ := s.GetMeta(ctx, docID)
	chunks, err := s.GetChunks(ctx, docID)
	if err != nil {
		return err
	}
	writePool := kvstore.GetWritePool()
	wc := writePool.Get()
	defer writePool.Put(wc)
	for _, ch := range chunks {
		if err := wc.Del(ctx, kvstore.ChunkKey(ch.ID)); err != nil {
			return err
		}
	}
	if err := wc.Del(ctx, kvstore.DocChunkIndexKey(docID)); err != nil {
		return err
	}
	if err := wc.Del(ctx, kvstore.DocMetaKey(docID)); err != nil {
		return err
	}
	if meta != nil && meta.ContentHash != "" {
		_ = wc.Del(ctx, kvstore.DocHashKey(meta.ContentHash))
	}

	readPool := kvstore.GetReadPool()
	rc := readPool.Get()
	defer readPool.Put(rc)
	raw, err := rc.Get(ctx, kvstore.DocIndexKey())
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return err
	}
	next := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != docID {
			next = append(next, id)
		}
	}
	indexBytes, err := json.Marshal(next)
	if err != nil {
		return err
	}
	return wc.SetOrMod(ctx, kvstore.DocIndexKey(), string(indexBytes))
}

func (s *kvDocumentStore) FindByContentHash(ctx context.Context, hash string) (*DocumentMeta, error) {
	pool := kvstore.GetReadPool()
	client := pool.Get()
	defer pool.Put(client)
	raw, err := client.Get(ctx, kvstore.DocHashKey(hash))
	if err != nil {
		return nil, err
	}
	if raw != nil {
		docID := strings.TrimSpace(string(raw))
		if docID != "" {
			return s.GetMeta(ctx, docID)
		}
	}
	// 兼容旧数据：无 hash 索引时线性扫描
	metas, err := s.ListMeta(ctx)
	if err != nil {
		return nil, err
	}
	for _, meta := range metas {
		if meta.ContentHash == hash {
			copyMeta := meta
			return &copyMeta, nil
		}
	}
	return nil, nil
}

func buildDocument(input IngestInput) *DocumentMeta {
	hash := docContentHash(input.Content)
	docID := makeDocID(input.SourcePath, hash)
	now := nowUnix()
	return &DocumentMeta{
		ID:          docID,
		Title:       input.Title,
		SourcePath:  input.SourcePath,
		SourceType:  input.SourceType,
		ContentHash: hash,
		CharCount:   len([]rune(input.Content)),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func buildChunks(docID string, drafts []ChunkDraft) []Chunk {
	chunks := make([]Chunk, 0, len(drafts))
	for _, d := range drafts {
		chunks = append(chunks, Chunk{
			ID:       makeChunkID(docID, d.Index),
			DocID:    docID,
			Index:    d.Index,
			Chapter:  d.Chapter,
			Content:  d.Content,
			StartPos: d.StartPos,
			EndPos:   d.EndPos,
		})
	}
	return chunks
}
