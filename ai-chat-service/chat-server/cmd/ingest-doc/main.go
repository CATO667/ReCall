package main

import (
	"ai-chat-service/chat-server/kbase"
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"ai-chat-service/pkg/db/kvstore"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	configPath := flag.String("config", "dev.config.yaml", "config file")
	filePath := flag.String("file", "", "single document file (.txt/.md)")
	dirPath := flag.String("dir", "", "directory to ingest recursively")
	maxKB := flag.Int("max-kb", 0, "only read first N KB from source file")
	sampleOut := flag.String("sample-out", "", "save truncated UTF-8 sample to this path")
	listDocs := flag.Bool("list", false, "list ingested documents")
	forceIngest := flag.Bool("force", false, "force re-ingest even if content hash exists")
	deleteDoc := flag.String("delete", "", "delete document by doc_id")
	showDoc := flag.String("show", "", "show document chunks by doc_id")
	flag.Parse()

	config.InitConfig(*configPath)
	cnf := config.GetConfig()
	kvstore.InitPools(cnf)

	embedder, _ := vector.NewFromConfig(cnf, nil)
	ingester := kbase.NewIngester(cnf, embedder)
	ctx := context.Background()

	switch {
	case *listDocs:
		metas, err := ingester.ListDocuments(ctx)
		if err != nil {
			log.Fatal(err)
		}
		if len(metas) == 0 {
			fmt.Println("no documents")
			return
		}
		for _, meta := range metas {
			fmt.Printf("%s\tchunks=%d\tchars=%d\ttitle=%q\tpath=%s\n",
				meta.ID, meta.ChunkCount, meta.CharCount, meta.Title, meta.SourcePath)
		}
	case *deleteDoc != "":
		if err := ingester.DeleteDocument(ctx, *deleteDoc); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("deleted doc %s\n", *deleteDoc)
	case *showDoc != "":
		meta, chunks, err := ingester.GetDocument(ctx, *showDoc)
		if err != nil {
			log.Fatal(err)
		}
		if meta == nil {
			fmt.Println("document not found")
			return
		}
		fmt.Printf("doc_id=%s title=%q chunks=%d path=%s\n", meta.ID, meta.Title, meta.ChunkCount, meta.SourcePath)
		for _, ch := range chunks {
			preview := ch.Content
			if len([]rune(preview)) > 80 {
				preview = string([]rune(preview)[:80]) + "..."
			}
			chapter := ch.Chapter
			if chapter == "" {
				chapter = "-"
			}
			fmt.Printf("  [%04d] chapter=%s %s\n", ch.Index, chapter, preview)
		}
	case *filePath != "":
		if *sampleOut != "" {
			maxBytes := 0
			if *maxKB > 0 {
				maxBytes = *maxKB * 1024
			}
			if _, err := kbase.SaveUTF8Sample(*filePath, *sampleOut, maxBytes); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("sample saved: %s\n", *sampleOut)
		}
		start := time.Now()
		var res *kbase.IngestResult
		var err error
		opts := kbase.IngestOptions{Force: *forceIngest}
		if *maxKB > 0 {
			res, err = ingester.IngestFileWithOptions(ctx, *filePath, *maxKB*1024, opts)
		} else {
			res, err = ingester.IngestFileWithOptions(ctx, *filePath, 0, opts)
		}
		if err != nil {
			log.Fatal(err)
		}
		printResult(res, time.Since(start))
	case *dirPath != "":
		files, err := collectFiles(*dirPath)
		if err != nil {
			log.Fatal(err)
		}
		if len(files) == 0 {
			fmt.Println("no supported files found")
			return
		}
		ok, skip, fail := 0, 0, 0
		for _, f := range files {
			res, err := ingester.IngestFileWithOptions(ctx, f, 0, kbase.IngestOptions{Force: *forceIngest})
			if err != nil {
				fail++
				log.Printf("ingest failed %s: %v", f, err)
				continue
			}
			if res.Skipped {
				skip++
			} else {
				ok++
			}
			printResult(res, 0)
		}
		fmt.Printf("batch done: success=%d skipped=%d failed=%d total=%d\n", ok, skip, fail, len(files))
	default:
		fmt.Fprintf(os.Stderr, "usage: ingest-doc --file path [--force] [--max-kb N] [--sample-out path] | --dir path [--force] | --list | --show doc_id | --delete doc_id\n")
		os.Exit(2)
	}
}

func collectFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".txt", ".md", ".markdown":
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func printResult(res *kbase.IngestResult, elapsed time.Duration) {
	if res.Skipped {
		fmt.Printf("skipped existing doc_id=%s title=%q chunks=%d\n",
			res.Document.ID, res.Document.Title, res.Document.ChunkCount)
		return
	}
	if elapsed > 0 {
		fmt.Printf("ingested doc_id=%s title=%q chunks=%d indexed=%d chars=%d elapsed=%s path=%s\n",
			res.Document.ID, res.Document.Title, len(res.Chunks), res.Indexed, res.Document.CharCount, elapsed.Round(time.Millisecond), res.Document.SourcePath)
		return
	}
	fmt.Printf("ingested doc_id=%s title=%q chunks=%d indexed=%d path=%s\n",
		res.Document.ID, res.Document.Title, len(res.Chunks), res.Indexed, res.Document.SourcePath)
}
