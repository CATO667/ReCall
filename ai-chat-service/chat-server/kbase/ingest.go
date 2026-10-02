package kbase

import (
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"context"
	"fmt"
	"strings"
	"sync"
)

type ChunkIndexer interface {
	IndexChunks(ctx context.Context, doc *DocumentMeta, chunks []Chunk, vectors [][]float32) error
	DeleteDoc(ctx context.Context, docID string) error
}

type noopChunkIndexer struct{}

func (n *noopChunkIndexer) IndexChunks(ctx context.Context, doc *DocumentMeta, chunks []Chunk, vectors [][]float32) error {
	return nil
}

func (n *noopChunkIndexer) DeleteDoc(ctx context.Context, docID string) error {
	return nil
}

func NewChunkIndexer(cnf *config.Config) ChunkIndexer {
	if cnf == nil || !cnf.KBase.Enabled || !cnf.Vector.Enabled {
		return &noopChunkIndexer{}
	}
	if vector.ProviderName(cnf) != vector.ProviderKvstore {
		return &noopChunkIndexer{}
	}
	return newKvstoreChunkIndexer(cnf)
}

type Ingester struct {
	store            DocumentStore
	parser           Parser
	chunker          Chunker
	indexer          ChunkIndexer
	embedder         vector.Embedder
	embedConcurrency int
}

func NewIngester(cnf *config.Config, embedder vector.Embedder) *Ingester {
	minSize := cnf.KBase.MinChunkSize
	if minSize <= 0 {
		minSize = 50
	}
	concurrency := cnf.KBase.EmbedConcurrency
	if concurrency <= 0 {
		concurrency = 8
	}
	chunkCfg := ChunkConfig{
		Strategy:     cnf.KBase.ChunkStrategy,
		ChunkSize:    cnf.KBase.ChunkSize,
		ChunkOverlap: cnf.KBase.ChunkOverlap,
		MinChunkSize: minSize,
	}
	return &Ingester{
		store:            NewDocumentStore(),
		parser:           NewParser(),
		chunker:          NewChunker(chunkCfg),
		indexer:          NewChunkIndexer(cnf),
		embedder:         embedder,
		embedConcurrency: concurrency,
	}
}

type IngestOptions struct {
	Force bool
}

func (ing *Ingester) IngestFile(ctx context.Context, path string) (*IngestResult, error) {
	return ing.IngestFileWithOptions(ctx, path, 0, IngestOptions{})
}

func (ing *Ingester) IngestFileWithLimit(ctx context.Context, path string, maxBytes int) (*IngestResult, error) {
	return ing.IngestFileWithOptions(ctx, path, maxBytes, IngestOptions{})
}

func (ing *Ingester) IngestFileWithOptions(ctx context.Context, path string, maxBytes int, opts IngestOptions) (*IngestResult, error) {
	parsed, err := ing.parser.ParseFileWithLimit(path, maxBytes)
	if err != nil {
		return nil, err
	}
	return ing.IngestWithOptions(ctx, IngestInput{
		Title:      parsed.Title,
		Content:    parsed.Content,
		SourcePath: path,
		SourceType: parsed.Type,
	}, opts)
}

func (ing *Ingester) Ingest(ctx context.Context, input IngestInput) (*IngestResult, error) {
	return ing.IngestWithOptions(ctx, input, IngestOptions{})
}

func (ing *Ingester) IngestWithOptions(ctx context.Context, input IngestInput, opts IngestOptions) (*IngestResult, error) {
	if strings.TrimSpace(input.Content) == "" {
		return nil, fmt.Errorf("empty content")
	}
	if input.Title == "" {
		input.Title = "untitled"
	}
	if input.SourceType == "" {
		input.SourceType = SourceTypeText
	}

	hash := docContentHash(input.Content)
	existing, err := ing.store.FindByContentHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if opts.Force {
			if err := ing.DeleteDocument(ctx, existing.ID); err != nil {
				return nil, fmt.Errorf("force re-ingest delete: %w", err)
			}
		} else {
			chunks, err := ing.store.GetChunks(ctx, existing.ID)
			if err != nil {
				return nil, err
			}
			return &IngestResult{
				Document: existing,
				Chunks:   chunks,
				Skipped:  true,
			}, nil
		}
	}

	meta := buildDocument(input)
	drafts := ing.chunker.Split(input.Content)
	chunks := buildChunks(meta.ID, drafts)
	meta.ChunkCount = len(chunks)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks produced after splitting")
	}

	var vectors [][]float32
	if ing.embedder != nil {
		vectors, err = embedChunksParallel(ctx, ing.embedder, chunks, ing.embedConcurrency)
		if err != nil {
			return nil, err
		}
	}
	if err := ing.store.Save(ctx, meta, chunks); err != nil {
		return nil, err
	}
	if err := ing.indexer.IndexChunks(ctx, meta, chunks, vectors); err != nil {
		_ = ing.store.Delete(ctx, meta.ID)
		return nil, fmt.Errorf("index vectors: %w", err)
	}
	InvalidateKeywordIndex()
	return &IngestResult{
		Document: meta,
		Chunks:   chunks,
		Indexed:  len(chunks),
	}, nil
}

func embedChunksParallel(ctx context.Context, embedder vector.Embedder, chunks []Chunk, concurrency int) ([][]float32, error) {
	if be, ok := embedder.(interface {
		EmbedBatch(context.Context, []string) ([][]float32, error)
	}); ok {
		return embedChunksBatch(ctx, be, chunks, 16)
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	vectors := make([][]float32, len(chunks))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, ch := range chunks {
		wg.Add(1)
		go func(i int, ch Chunk) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			vec, err := embedder.Embed(ctx, ch.Content)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("embed chunk %s: %w", ch.ID, err)
				}
				mu.Unlock()
				return
			}
			vectors[i] = vec
		}(i, ch)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return vectors, nil
}

func embedChunksBatch(ctx context.Context, embedder interface {
	EmbedBatch(context.Context, []string) ([][]float32, error)
}, chunks []Chunk, batchSize int) ([][]float32, error) {
	if batchSize <= 0 {
		batchSize = 16
	}
	vectors := make([][]float32, len(chunks))
	for start := 0; start < len(chunks); start += batchSize {
		end := start + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		texts := make([]string, end-start)
		for i, ch := range chunks[start:end] {
			texts[i] = ch.Content
		}
		batch, err := embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", start, end, err)
		}
		copy(vectors[start:end], batch)
	}
	return vectors, nil
}

func (ing *Ingester) ListDocuments(ctx context.Context) ([]DocumentMeta, error) {
	return ing.store.ListMeta(ctx)
}

func (ing *Ingester) GetDocument(ctx context.Context, docID string) (*DocumentMeta, []Chunk, error) {
	meta, err := ing.store.GetMeta(ctx, docID)
	if err != nil {
		return nil, nil, err
	}
	if meta == nil {
		return nil, nil, nil
	}
	chunks, err := ing.store.GetChunks(ctx, docID)
	if err != nil {
		return nil, nil, err
	}
	return meta, chunks, nil
}

func (ing *Ingester) DeleteDocument(ctx context.Context, docID string) error {
	if err := ing.indexer.DeleteDoc(ctx, docID); err != nil {
		return err
	}
	return ing.store.Delete(ctx, docID)
}
