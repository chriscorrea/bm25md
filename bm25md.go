// Package bm25md provides BM25-based search functionality with Markdown field awareness.
//
// This implements a field-weighted variant of the BM25 (Best Matching 25) ranking algorithm
// that gives different weights to content based on its semantic importance
// in Markdown documents (headers, bold text, body content, etc.).
//
// The algorithm handles multiple fields, where each field can have its own weight,
// allowing for more nuanced ranking based on where terms appear in the document structure.
package bm25md

import (
	"log/slog"
	"math"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// tokenRegex is compiled once for efficient tokenization
var tokenRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// Field represents a specific markdown field type with its weight
type Field string

const (
	FieldH1     Field = "h1"
	FieldH2     Field = "h2"
	FieldH3     Field = "h3"
	FieldH4     Field = "h4"
	FieldH5     Field = "h5"
	FieldH6     Field = "h6"
	FieldBold   Field = "bold"
	FieldItalic Field = "italic"
	FieldCode   Field = "code"
	FieldBody   Field = "body"
)

// DefaultFieldWeights provides sensible default weights for markdown fields
var DefaultFieldWeights = map[Field]float64{
	FieldH1:     5.0,
	FieldH2:     3.0,
	FieldH3:     2.0,
	FieldH4:     2.0,
	FieldH5:     2.0,
	FieldH6:     2.0,
	FieldBold:   1.5,
	FieldItalic: 1.2,
	FieldCode:   0.8,
	FieldBody:   1.0,
}

// Document represents a parsed document with field-separated content
type Document struct {
	ID       int              // document identifier
	Fields   map[Field]string // content separated by field type
	Original string           // original document text
}

// BM25Parameters holds the tuning parameters for BM25 algorithm
type BM25Parameters struct {
	K1 float64 // controls term frequency saturation
	B  float64 // controls length normalization
}

// DefaultBM25Parameters returns recommended BM25 parameters
func DefaultBM25Parameters() BM25Parameters {
	return BM25Parameters{
		K1: 1.2,
		B:  0.75,
	}
}

// DefaultFieldBM25Parameters returns field-specific BM25 parameters optimized for each field type
func DefaultFieldBM25Parameters() map[Field]BM25Parameters {
	return map[Field]BM25Parameters{
		// headers: quick saturation, strong length penalty
		FieldH1: {K1: 1.0, B: 0.9},
		FieldH2: {K1: 1.1, B: 0.9},
		FieldH3: {K1: 1.1, B: 0.87},
		FieldH4: {K1: 1.2, B: 0.85},
		FieldH5: {K1: 1.2, B: 0.83},
		FieldH6: {K1: 1.2, B: 0.8},

		// emphasis: quick saturation for short emphasized text
		FieldBold:   {K1: 0.9, B: 0.85}, // bold text should saturate quickly
		FieldItalic: {K1: 0.9, B: 0.85}, // emphasis text should saturate quickly

		// code: medium saturation, lenient on length
		FieldCode: {K1: 1.2, B: 0.5},

		// body: higher saturation for longer content–term frequency matters more
		FieldBody: {K1: 1.5, B: 0.75},
	}
}

// Tokenizer defines the interface for text tokenization
type Tokenizer interface {
	Tokenize(text string) []string
}

// DefaultTokenizer implements a basic default tokenizer
type DefaultTokenizer struct{}

// Tokenize implements the Tokenizer interface
func (t DefaultTokenizer) Tokenize(text string) []string {
	if text == "" {
		return []string{}
	}

	// convert to lowercase
	text = strings.ToLower(text)

	// split on non-alphanumeric characters
	tokens := tokenRegex.Split(text, -1)

	// filter out empty and short tokens
	var filtered []string
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if len(token) >= 3 { // skip very short words
			filtered = append(filtered, token)
		}
	}

	return filtered
}

// TokenizerFunc is a func adapter that allows using functions as Tokenizers
type TokenizerFunc func(string) []string

// Tokenize implements the Tokenizer interface for function types
func (f TokenizerFunc) Tokenize(text string) []string {
	return f(text)
}

// fieldBM25 manages BM25 scoring for a single field
type fieldBM25 struct {
	field           Field
	weight          float64
	params          BM25Parameters   // field-specific BM25 parameters
	termFrequencies []map[string]int // term frequencies per doc
	docFrequencies  map[string]int   // doc frequencies per term
	docLengths      []int            // length of each doc
	avgDocLength    float64          // average doc length
	totalDocs       int              // total number of docs
}

// newFieldBM25 creates a new field-specific BM25 scorer
func newFieldBM25(field Field, weight float64, params BM25Parameters) *fieldBM25 {
	return &fieldBM25{
		field:           field,
		weight:          weight,
		params:          params,
		termFrequencies: make([]map[string]int, 0),
		docFrequencies:  make(map[string]int),
		docLengths:      make([]int, 0),
	}
}

// addDocument indexes pre-tokenized content for this field
func (f *fieldBM25) addDocument(tokens []string) {

	// calculate term frequencies
	tf := make(map[string]int)
	for _, token := range tokens {
		tf[token]++
	}
	f.termFrequencies = append(f.termFrequencies, tf)

	// update doc frequencies
	seen := make(map[string]bool)
	for _, token := range tokens {
		if !seen[token] {
			f.docFrequencies[token]++
			seen[token] = true
		}
	}

	// store doc length
	f.docLengths = append(f.docLengths, len(tokens))
	f.totalDocs++

	// update average doc length
	totalLength := 0
	for _, length := range f.docLengths {
		totalLength += length
	}
	if f.totalDocs > 0 {
		f.avgDocLength = float64(totalLength) / float64(f.totalDocs)
	}
}

// score calculates BM25 score for a query on a specific document
func (f *fieldBM25) score(queryTerms []string, docIndex int) float64 {
	if docIndex < 0 || docIndex >= len(f.termFrequencies) {
		return 0.0
	}

	score := 0.0
	docTF := f.termFrequencies[docIndex]
	docLen := float64(f.docLengths[docIndex])

	for _, term := range queryTerms {
		tf := float64(docTF[term])
		if tf == 0 {
			continue
		}

		// calculate IDF
		df := float64(f.docFrequencies[term])
		if df == 0 {
			continue
		}
		// standard BM25 IDF formula: log((N - df + 0.5) / (df + 0.5))
		// where N is total documents, df is doc frequency
		idf := math.Log((float64(f.totalDocs) - df + 0.5) / (df + 0.5))
		if idf < 0 {
			idf = 0 // prevent negative IDF for small corpora
		}

		// calculate normalized term frequency using field-specific parameters
		normTF := tf * (f.params.K1 + 1) / (tf + f.params.K1*(1-f.params.B+f.params.B*docLen/f.avgDocLength))

		// accumulate score
		score += idf * normTF
	}

	// apply field weight
	return score * f.weight
}

// Corpus manages the BM25md search index for a corpus
type Corpus struct {
	documents    []Document
	fieldScorers map[Field]*fieldBM25
	fieldWeights map[Field]float64
	params       BM25Parameters
	tokenizer    Tokenizer
	fieldParams  map[Field]BM25Parameters // per-field BM25 parameters
}

// CorpusOption defines a function that configures a corpus
type CorpusOption func(*Corpus)

// WithTokenizer sets a custom tokenizer for the corpus
func WithTokenizer(tokenizer Tokenizer) CorpusOption {
	return func(c *Corpus) {
		c.tokenizer = tokenizer
	}
}

// WithFieldWeights sets custom field weights for the corpus
func WithFieldWeights(fieldWeights map[Field]float64) CorpusOption {
	return func(c *Corpus) {
		if fieldWeights != nil {
			c.fieldWeights = fieldWeights
		}
	}
}

// WithBM25Params sets custom BM25 parameters for the corpus
func WithBM25Params(params BM25Parameters) CorpusOption {
	return func(c *Corpus) {
		c.params = params
	}
}

// WithFieldParams sets per-field BM25 parameters (BM25F mode)
func WithFieldParams(fieldParams map[Field]BM25Parameters) CorpusOption {
	return func(c *Corpus) {
		if fieldParams != nil {
			c.fieldParams = fieldParams
		}
	}
}

// buildFieldScorers builds the field scorers based on current corpus configuration
func (c *Corpus) buildFieldScorers() {
	c.fieldScorers = make(map[Field]*fieldBM25)
	for field, weight := range c.fieldWeights {
		// use field-specific parameters if available, otherwise default
		params := c.params
		if c.fieldParams != nil {
			if fieldParam, exists := c.fieldParams[field]; exists {
				params = fieldParam
			}
		}
		c.fieldScorers[field] = newFieldBM25(field, weight, params)
	}
}

// NewCorpus creates a new BM25md corpus with optional configuration
func NewCorpus(opts ...CorpusOption) *Corpus {
	// init with defaults (see above)
	corpus := &Corpus{
		documents:    make([]Document, 0),
		fieldWeights: DefaultFieldWeights,
		params:       DefaultBM25Parameters(),
		tokenizer:    DefaultTokenizer{},
	}

	// apply user options
	for _, opt := range opts {
		opt(corpus)
	}

	// build field scorers
	corpus.buildFieldScorers()

	return corpus
}

// AddDocument adds a document to the corpus
func (c *Corpus) AddDocument(doc Document) {
	doc.ID = len(c.documents)
	c.documents = append(c.documents, doc)

	// index content in each field
	for field, scorer := range c.fieldScorers {
		content := doc.Fields[field]
		tokens := c.tokenizer.Tokenize(content)
		scorer.addDocument(tokens)
	}

	slog.Debug("Added document to BM25md corpus", "docID", doc.ID, "fields", len(doc.Fields))
}

// Score calculates the BM25md score for a query against a specific document
func (c *Corpus) Score(query string, docIndex int) float64 {
	queryTerms := c.tokenizer.Tokenize(query)
	return c.scoreWithTokens(queryTerms, docIndex)
}

// This implements a BM25F formula which combines term frequencies across fields
func (c *Corpus) scoreWithTokens(queryTerms []string, docIndex int) float64 {
	if docIndex < 0 || docIndex >= len(c.documents) {
		return 0.0
	}

	if len(queryTerms) == 0 {
		return 0.0
	}

	totalScore := 0.0
	totalDocs := len(c.documents)

	// calculate score per term across all fields
	for _, term := range queryTerms {
		docFreq := 0
		for i := 0; i < len(c.documents); i++ {
			termFound := false
			for _, scorer := range c.fieldScorers {
				if i < len(scorer.termFrequencies) {
					if scorer.termFrequencies[i][term] > 0 {
						termFound = true
						break
					}
				}
			}
			if termFound {
				docFreq++
			}
		}
		if docFreq == 0 {
			continue
		}

		idf := math.Log((float64(totalDocs) - float64(docFreq) + 0.5) / (float64(docFreq) + 0.5))
		if idf < 0 {
			idf = 0 // prevent negative IDF for small corpora
		}

		// calculate weighted term frequency across all fields (true BM25F)
		weightedTF := 0.0
		for field, scorer := range c.fieldScorers {
			if docIndex < len(scorer.termFrequencies) {
				tf := float64(scorer.termFrequencies[docIndex][term])
				if tf > 0 {
					weight := c.fieldWeights[field]
					weightedTF += weight * tf
				}
			}
		}

		// apply BM25F normalization with combined term frequency
		if weightedTF > 0 {
			// use default K1=1.2 for the combined normalization
			k1 := 1.2
			normTF := weightedTF * (k1 + 1) / (weightedTF + k1)
			termScore := idf * normTF
			totalScore += termScore
		}
	}

	return totalScore
}

// SearchResult represents a document with its relevance score
type SearchResult struct {
	Document Document
	Score    float64
	Index    int
}

// Search performs a BM25md search and returns ranked results
func (c *Corpus) Search(query string, limit int) []SearchResult {
	queryTerms := c.tokenizer.Tokenize(query)
	if len(queryTerms) == 0 {
		return []SearchResult{}
	}

	// for small corpora, use sequential processing to avoid overhead
	if len(c.documents) < 100 {
		return c.searchSequential(queryTerms, limit)
	}

	return c.searchParallel(queryTerms, limit)
}

// searchSequential performs sequential document scoring for small corpora
func (c *Corpus) searchSequential(queryTerms []string, limit int) []SearchResult {
	results := make([]SearchResult, 0, len(c.documents))

	// score all documents sequentially
	for i, doc := range c.documents {
		score := c.scoreWithTokens(queryTerms, i)
		if score > 0 {
			results = append(results, SearchResult{
				Document: doc,
				Score:    score,
				Index:    i,
			})
		}
	}

	// sort by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// searchParallel performs parallel document scoring for large collections
func (c *Corpus) searchParallel(queryTerms []string, limit int) []SearchResult {
	numWorkers := runtime.NumCPU()
	if numWorkers > len(c.documents) {
		numWorkers = len(c.documents)
	}

	// create channels for work distribution/result collection
	docChan := make(chan int, len(c.documents))
	resultsChan := make(chan SearchResult, len(c.documents))

	// start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for docIndex := range docChan {
				score := c.scoreWithTokens(queryTerms, docIndex)
				if score > 0 {
					resultsChan <- SearchResult{
						Document: c.documents[docIndex],
						Score:    score,
						Index:    docIndex,
					}
				}
			}
		}()
	}

	// send work to workers
	go func() {
		defer close(docChan)
		for i := range c.documents {
			docChan <- i
		}
	}()

	// start result collection goroutine
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// collect results
	results := make([]SearchResult, 0, len(c.documents))
	for result := range resultsChan {
		results = append(results, result)
	}

	// sort by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}
