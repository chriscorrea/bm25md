package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chriscorrea/bm25md"
)

func main() {

	// custom field weights determine the importance of a search term match based on its location.
	legalDocWeights := map[bm25md.Field]float64{
		bm25md.FieldH1:     5.0, // very strongly weighted
		bm25md.FieldH2:     3.0, // strongly weighted
		bm25md.FieldH3:     2.0, // moderately strongly weighted
		bm25md.FieldBold:   1.5, // somewhat strongly weighted
		bm25md.FieldItalic: 1.2, // somewhat strongly weighted
		bm25md.FieldCode:   0.8, // lightly weighted for code blocks
		bm25md.FieldBody:   1.0, // baseline weight
	}

	// custom BM25 parameters tune the relevance scoring algorithm
	legalParams := bm25md.BM25Parameters{
		// K1 controls how quickly a term's score saturates with repeated occurrences
		K1: 1.5, // moderate term frequency saturation

		// B controls the document length penalty
		B: 0.8, // somewhat high length normalization
	}

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

	// create corpus with custom weights and parser
	corpus := bm25md.NewCorpus(
		bm25md.WithFieldWeights(legalDocWeights),
		bm25md.WithBM25Params(legalParams),
	)
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

	fmt.Printf("Indexed %d paragraphs from document (with custom weights)\n\n", len(docs))

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
