# BM25md - Field-Aware BM25 for Markdown

[![Go Version](https://img.shields.io/github/go-mod/go-version/chriscorrea/slop)](go.mod)
[![Go Reference](https://pkg.go.dev/badge/github.com/chriscorrea/bm25md.svg)](https://pkg.go.dev/github.com/chriscorrea/bm25md)
[![Go Report Card](https://goreportcard.com/badge/github.com/chriscorrea/bm25md)](https://goreportcard.com/report/github.com/chriscorrea/bm25md)
[![Tests](https://github.com/chriscorrea/bm25md/actions/workflows/test.yml/badge.svg)](https://github.com/chriscorrea/bm25md/actions/workflows/test.yml)


BM25md is a Go implementation of the BM25 text ranking algorithm with field-aware scoring for [markdown](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax) documents. It augments traditional BM25 by assigning different weights to content based on its semantic importance in markdown structure (headers, bold text, code blocks, etc.).

## Installation

```bash
go get github.com/chriscorrea/bm25md
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/chriscorrea/bm25md"
)

func main() {
    // create a corpus with default config
    corpus := bm25md.NewCorpus()
    
    // create a parser for markdown documents
    parser := bm25md.NewMarkdownFieldParser()
    
    // add documents
    docs := []string{
        "# Introduction\nThis is a **key concept** in information retrieval.",
        "## Details\nThe algorithm uses `code examples` to demonstrate usage.",
        "### Summary\nBM25 is effective for ranking documents.",
    }
    
    for _, content := range docs {
        fields := parser.ParseDocument(content)
        corpus.AddDocument(bm25md.Document{
            Fields:   fields,
            Original: content,
        })
    }
    
    // search
    results := corpus.Search("key concept", 10)
    
    for _, result := range results {
        fmt.Printf("Score: %.3f, Document: %s\n", result.Score, result.Document.Original)
    }
}
```

## Custom Configuration

The functional options API provides clean, extensible configuration:

```go
// custom field weights
weights := map[bm25md.Field]float64{
    bm25md.FieldH1:   10.0,  // very heavily weight H1 headers
    bm25md.FieldCode: 2.0,   // increase code importance
    bm25md.FieldBody: 1.0,   
}

// custom BM25 parameters
params := bm25md.BM25Parameters{
    K1: 1.5,  // term frequency saturation
    B:  0.5,  // doc length normalization
}

// Create corpus with custom configuration
corpus := bm25md.NewCorpus(
    bm25md.WithFieldWeights(weights),
    bm25md.WithBM25Params(params),
)
```

Note that, for advanced use cases, you can specify different BM25 parameters for each field:

```go
fieldParams := map[bm25md.Field]bm25md.BM25Parameters{
    bm25md.FieldH1:   {K1: 0.8, B: 0.9}, // long headers are penalized
    bm25md.FieldBody: {K1: 1.5, B: 0.75}, // higher saturation for body text
    bm25md.FieldCode: {K1: 1.2, B: 0.5},  // lenient on code block length
}

corpus := bm25md.NewCorpus(bm25md.WithFieldParams(fieldParams))
```

### Custom Tokenizers

The default tokenizer is very simple. You can implement custom tokenization to apply stemming, normalization, or domain-specific processing.

Using a function-based tokenizer provides a lightweight approach:

```go
customStemmingTokenizer := func(text string) []string {
    tokens := strings.Fields(strings.ToLower(text))
    // apply your custom stemming/normalization logic here
    return applyStemming(tokens)
}

corpus := bm25md.NewCorpus(bm25md.WithTokenizer(bm25md.TokenizerFunc(customStemmingTokenizer)))
```

Alternatively, you can implement the Tokenizer interface:

```go
type MyTokenizer struct{}
func (t MyTokenizer) Tokenize(text string) []string {
    // apply your custom stemming/normalization logic here
    return customTokenize(text)
}

corpus := bm25md.NewCorpus(bm25md.WithTokenizer(MyTokenizer{}))
```

## Dependencies

BM25md leverages [goldmark](https://github.com/yuin/goldmark) for AST-based markdown parsing.

## Contributing

Contributions and issues are welcome – please see the [issues page](https://github.com/chriscorrea/bmd25md/issues).

## License

This project is licensed under the [BSD-3 License](LICENSE).