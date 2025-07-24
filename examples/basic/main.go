package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chriscorrea/bm25md"
)

func main() {
	// read the document
	dataPath := filepath.Join("..", "data", "habeas_corpus.md")
	content, err := os.ReadFile(dataPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// chunk by paragraph and remove empty ones
	candidates := strings.Split(string(content), "\n\n")
	var docs []string
	for _, p := range candidates {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			// only add non-empty paragraphs
			docs = append(docs, trimmed)
		}
	}

	// create corpus and parser  
	corpus := bm25md.NewCorpus()
	parser := bm25md.NewMarkdownFieldParser()

	// index documents
	for i, doc := range docs {
		fields := parser.ParseDocument(doc)
		corpus.AddDocument(bm25md.Document{
			ID:       i,
			Fields:   fields,
			Original: doc,
		})
	}

	fmt.Printf("Indexed %d paragraphs from document\n\n", len(docs))

	// example queries (try others!)
	queries := []string{
		"habeas corpus",
		"federal court",
		"constitutional rights",
	}

	for _, query := range queries {
		fmt.Printf("Query: %q\n", query)
		results := corpus.Search(query, 3)

		for i, result := range results {
			// create preview from document
			preview := result.Document.Original
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			preview = strings.ReplaceAll(preview, "\n", " ")

			fmt.Printf("  %d. Score: %.2f\tContent: %s\n", i+1, result.Score, preview)
		}
		fmt.Println()
	}
}
