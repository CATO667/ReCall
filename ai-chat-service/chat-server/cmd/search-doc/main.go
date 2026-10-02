package main

import (
	"ai-chat-service/chat-server/kbase"
	"ai-chat-service/chat-server/vector"
	"ai-chat-service/pkg/config"
	"context"
	"flag"
	"fmt"
	"log"
)

func main() {
	configPath := flag.String("config", "dev.config.yaml", "config file")
	query := flag.String("query", "", "search query")
	topK := flag.Int("top-k", 0, "override top k")
	flag.Parse()

	if *query == "" {
		log.Fatal("usage: search-doc --query=\"your question\"")
	}

	config.InitConfig(*configPath)
	cnf := config.GetConfig()
	embedder, _ := vector.NewFromConfig(cnf, nil)
	searcher := kbase.NewDocSearcher(cnf)
	if searcher == nil {
		log.Fatal("doc searcher not enabled; check kbase.enabled and vector settings")
	}

	ctx := context.Background()
	vec, err := embedder.Embed(ctx, *query)
	if err != nil {
		log.Fatal(err)
	}
	hits, err := searcher.Search(ctx, vec, *topK)
	if err != nil {
		log.Fatal(err)
	}
	if len(hits) == 0 {
		fmt.Println("no hits")
		return
	}
	for i, hit := range hits {
		preview := hit.Content
		if len([]rune(preview)) > 120 {
			preview = string([]rune(preview)[:120]) + "..."
		}
		chapter := hit.Chapter
		if chapter == "" {
			chapter = "-"
		}
		fmt.Printf("[%d] score=%.4f title=%q chapter=%s\n%s\n\n", i+1, hit.Score, hit.Title, chapter, preview)
	}
}
