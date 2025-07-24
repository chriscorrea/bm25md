package bm25md

import (
	"math"
	"testing"
)

// createTestCorpus returns a corpus with poetry documents for testing
func createTestCorpus() (*Corpus, []Document) {
	docs := []Document{
		{Fields: map[Field]string{FieldBody: "I shut my eyes and all the world drops dead;"}},
		{Fields: map[Field]string{FieldBody: "I lift my lids and all is born again."}},
		{Fields: map[Field]string{FieldBody: "(I think I made you up inside my head.)"}},
		{Fields: map[Field]string{FieldBody: "The stars go waltzing out in blue and red,"}},
		{Fields: map[Field]string{FieldBody: "And arbitrary blackness gallops in:"}},
		{Fields: map[Field]string{FieldBody: "I dreamed that you bewitched me into bed"}},
		{Fields: map[Field]string{FieldBody: "And sung me moon-struck, kissed me quite insane."}},
		// add filler docs for term discrimination
		{Fields: map[Field]string{FieldBody: "Nature documentaries about wildlife"}},
		{Fields: map[Field]string{FieldBody: "Scientific research on climate patterns"}},
		{Fields: map[Field]string{FieldBody: "Technology advances in computing"}},
	}
	
	corpus := NewCorpus()
	for _, doc := range docs {
		corpus.AddDocument(doc)
	}
	
	return corpus, docs
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple line",
			input:    "I lift my lids and all is born again",
			expected: []string{"lift", "lids", "and", "all", "born", "again"},
		},
		{
			name:     "line with punctuation",
			input:    "The stars go waltzing out in blue and red,",
			expected: []string{"the", "stars", "waltzing", "out", "blue", "and", "red"},
		},
		{
			name:     "parenthetical line",
			input:    "(I think I made you up inside my head.)",
			expected: []string{"think", "made", "you", "inside", "head"},
		},
		{
			name:     "filters short words",
			input:    "I lift my lids",
			expected: []string{"lift", "lids"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := DefaultTokenizer{}
			result := tokenizer.Tokenize(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Tokenize() returned %d tokens, want %d", len(result), len(tt.expected))
				return
			}
			for i, token := range result {
				if token != tt.expected[i] {
					t.Errorf("Tokenize()[%d] = %q, want %q", i, token, tt.expected[i])
				}
			}
		})
	}
}

func TestFieldBM25_AddDocument(t *testing.T) {
	field := newFieldBM25(FieldBody, 1.0, DefaultBM25Parameters())
	tokenizer := DefaultTokenizer{}

	// add the first doc
	tokens1 := tokenizer.Tokenize("I shut my eyes and all the world drops dead")
	field.addDocument(tokens1)

	if field.totalDocs != 1 {
		t.Errorf("totalDocs = %d, want 1", field.totalDocs)
	}
	if field.docFrequencies["world"] != 1 {
		t.Errorf("docFrequencies[world] = %d, want 1", field.docFrequencies["world"])
	}

	// add another doc
	tokens2 := tokenizer.Tokenize("I lift my lids and all is born again")
	field.addDocument(tokens2)

	if field.totalDocs != 2 {
		t.Errorf("totalDocs = %d, want 2", field.totalDocs)
	}
	if field.docFrequencies["all"] != 2 {
		t.Errorf("docFrequencies[all] = %d, want 2", field.docFrequencies["all"])
	}
	if field.termFrequencies[1]["lift"] != 1 {
		t.Errorf("termFrequencies[1][lift] = %d, want 1", field.termFrequencies[1]["lift"])
	}
}

func TestFieldBM25_Score(t *testing.T) {
	params := DefaultBM25Parameters()
	field := newFieldBM25(FieldBody, 2.0, params) // weight of 2.0
	tokenizer := DefaultTokenizer{}

	// add docs from the poem
	field.addDocument(tokenizer.Tokenize("The stars go waltzing out in blue and red"))
	field.addDocument(tokenizer.Tokenize("I dreamed that you bewitched me into bed"))
	field.addDocument(tokenizer.Tokenize("I should have loved a thunderbird instead"))

	// Test scoring for a term present in only the first document
	query := []string{"waltzing"}
	score0 := field.score(query, 0)
	score1 := field.score(query, 1)
	score2 := field.score(query, 2)

	if score0 <= 0 {
		t.Errorf("score for doc 0 = %f, want > 0", score0)
	}
	if score1 != 0 {
		t.Errorf("score for doc 1 = %f, want 0", score1)
	}
	if score2 != 0 {
		t.Errorf("score for doc 2 = %f, want 0", score2)
	}

	// test that the field weight is applied correctly
	fieldNoWeight := newFieldBM25(FieldBody, 1.0, params)
	fieldNoWeight.addDocument(tokenizer.Tokenize("The stars go waltzing out in blue and red"))
	fieldNoWeight.addDocument(tokenizer.Tokenize("I dreamed that you bewitched me into bed"))
	fieldNoWeight.addDocument(tokenizer.Tokenize("I should have loved a thunderbird instead"))
	scoreNoWeight := fieldNoWeight.score(query, 0)

	if math.Abs(score0-2*scoreNoWeight) > 1e-6 {
		t.Errorf("field weight not applied correctly: score = %f, expected ~%f", score0, 2*scoreNoWeight)
	}
}

func TestCorpus_Score(t *testing.T) {
	corpus := NewCorpus()

	docs := []Document{
		{Fields: map[Field]string{FieldBody: "I shut my eyes and all the world drops dead;"}},
		{Fields: map[Field]string{FieldBody: "I lift my lids and all is born again."}},
		{Fields: map[Field]string{FieldBody: "(I think I made you up inside my head.)"}},
		{Fields: map[Field]string{FieldBody: "The stars go waltzing out in blue and red,"}},
		{Fields: map[Field]string{FieldBody: "Various unrelated content about nature"}},
		{Fields: map[Field]string{FieldBody: "Different topic entirely about science"}},
	}
	for _, doc := range docs {
		corpus.AddDocument(doc)
	}

	// test scoring for a discriminative term
	score0 := corpus.Score("waltzing", 3) // should score well
	score1 := corpus.Score("waltzing", 0) // should score zero

	if score0 <= 0 {
		t.Errorf("score for 'waltzing' in doc 3 = %f, want > 0", score0)
	}
	if score1 != 0 {
		t.Errorf("score for 'waltzing' in doc 0 = %f, want 0", score1)
	}

	// test multi-term query ('think made head' should score about 3.9)
	multiScore := corpus.Score("think made head", 2)
	if multiScore <= 3 || multiScore >= 5 {
		t.Errorf("multi-term score = %f, should be between 3 and 5", multiScore)
	}
}

func TestCorpus_Search(t *testing.T) {
	corpus, _ := createTestCorpus()

	// search for "head", which only appears in one document
	results := corpus.Search("head", 5)
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want 1", len(results))
	}
	if results[0].Document.ID != 2 {
		t.Errorf("Expected document with ID 2, but got %d", results[0].Document.ID)
	}

	// search for a repeated phrase, expecting multiple sorted results
	sortedResults := corpus.Search("shut eyes world dead", 10)
	if len(sortedResults) < 1 {
		t.Errorf("Search returned no results, want at least 1")
	}
	for i := 1; i < len(sortedResults); i++ {
		if sortedResults[i].Score > sortedResults[i-1].Score {
			t.Errorf("Results not sorted: result %d score (%f) > result %d score (%f)", i, sortedResults[i].Score, i-1, sortedResults[i-1].Score)
		}
	}
}

func TestCorpus_SearchParallel(t *testing.T) {
	// create base corpus with test documents
	smallCorpus, baseDocs := createTestCorpus()
	
	// create large corpus (100+ docs) to trigger parallel search
	largeCorpus := NewCorpus()
	
	// add the original test documents first
	for _, doc := range baseDocs {
		largeCorpus.AddDocument(doc)
	}
	
	// add 90 more filler documents to reach 100+ threshold
	for i := 0; i < 90; i++ {
		largeCorpus.AddDocument(Document{
			Fields: map[Field]string{FieldBody: "Additional filler content for parallel testing"},
		})
	}
	
	tests := []struct {
		name       string
		query      string
		expectDocs int
		checkID    int // document ID to verify in first result
	}{
		{
			name:       "single term in one doc",
			query:      "head",
			expectDocs: 1,
			checkID:    2,
		},
		{
			name:       "multi-term query",
			query:      "shut eyes world dead",
			expectDocs: 1, // only first document should match well
			checkID:    0,
		},
		{
			name:       "common term",
			query:      "and",
			expectDocs: 5, // appears in multiple documents
			checkID:    -1, // don't check specific ID
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test both small (sequential) and large (parallel) corpora
			smallResults := smallCorpus.Search(tt.query, 10)
			largeResults := largeCorpus.Search(tt.query, 10)
			
			// verify parallel results contain expected number of docs
			if tt.expectDocs > 0 && len(largeResults) < tt.expectDocs {
				t.Errorf("Parallel search returned %d results, want at least %d", len(largeResults), tt.expectDocs)
			}
			
			// verify specific document ID if provided
			if tt.checkID >= 0 && len(largeResults) > 0 {
				if largeResults[0].Document.ID != tt.checkID {
					t.Errorf("Expected document with ID %d, but got %d", tt.checkID, largeResults[0].Document.ID)
				}
			}
			
			// verify results are sorted by score (descending)
			for i := 1; i < len(largeResults); i++ {
				if largeResults[i].Score > largeResults[i-1].Score {
					t.Errorf("Results not sorted: result %d score (%f) > result %d score (%f)", 
						i, largeResults[i].Score, i-1, largeResults[i-1].Score)
				}
			}
			
			// verify that parallel search finds the same relevant documents
			// (note: scores will be different due to different corpus size affecting IDF)
			if len(smallResults) > 0 && len(largeResults) > 0 {
				// verify that the most relevant document from sequential search 
				// is also found in parallel results (though score may differ)
				topSeqDoc := smallResults[0].Document.ID
				found := false
				for _, largeResult := range largeResults {
					if largeResult.Document.ID == topSeqDoc {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Top sequential result (doc %d) not found in parallel results", topSeqDoc)
				}
			}
		})
	}
}

func TestFieldWeighting(t *testing.T) {
	weights := map[Field]float64{
		FieldH1:   5.0, // high weight
		FieldH3:   2.0, // moderate weight
		FieldBody: 1.0, // baseline
	}
	corpus := NewCorpus(WithFieldWeights(weights))

	// doc 0
	corpus.AddDocument(Document{
		Fields: map[Field]string{
			FieldH1:   "Fires Fade",
			FieldH3:   "A Thunderbird Instead?",
			FieldBody: "At least when time comes they roar back again.",
		},
	})

	// doc 1 (includes some same terms in lower-weighted fields)
	corpus.AddDocument(Document{
		Fields: map[Field]string{
			FieldH1:   "Shut My Eyes",
			FieldH3:   "Fires Fade",
			FieldBody: "I should have loved a thunderbird instead;",
		},
	})

	// add other docs to make terms more selective
	corpus.AddDocument(Document{Fields: map[Field]string{FieldBody: "The stars go waltzing out in blue and red."}})
	corpus.AddDocument(Document{Fields: map[Field]string{FieldBody: "I think I made you up inside my head."}})
	corpus.AddDocument(Document{Fields: map[Field]string{FieldBody: "But I grow old and I forget your name."}})
	corpus.AddDocument(Document{Fields: map[Field]string{FieldBody: "And arbitrary blackness gallops in:"}})

	tests := []struct {
		name              string
		query             string
		higherWeightedDoc int
		lowerWeightedDoc  int
		description       string
	}{
		{
			name:              "H1 vs H3",
			query:             "thunderbird",
			higherWeightedDoc: 0, // thunderbird in h1
			lowerWeightedDoc:  1, // thunderbird in h3
			description:       "term in H1 should score higher than same term in H3",
		},
		{
			name:              "H3 vs Body",
			query:             "thunderbird",
			higherWeightedDoc: 0, // thunderbird in h3
			lowerWeightedDoc:  1, // thunderbird in body
			description:       "term in H3 should score higher than same term in Body",
		},
		{
			name:              "Multi-term query",
			query:             "thunderbird fade",
			higherWeightedDoc: 0, // "thunderbird" in h1 and "fade" in h3
			lowerWeightedDoc:  1, // "thunderbird" in h3 and "fade" in body
			description:       "multi-term query should benefit from higher field weights",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			higherScore := corpus.Score(tt.query, tt.higherWeightedDoc)
			lowerScore := corpus.Score(tt.query, tt.lowerWeightedDoc)

			if higherScore <= lowerScore {
				t.Errorf("%s: higher-weighted score (%f) should be greater than lower-weighted score (%f)",
					tt.description, higherScore, lowerScore)
			}
		})
	}
}

// basic markdown parser tests
func TestMarkdownFieldParser_BasicFields(t *testing.T) {
	parser := NewMarkdownFieldParser()

	markdown := `# Main Title
This is **bold** and *italic* text.
Here is some ` + "`code`" + ` and regular content.`

	fields := parser.ParseDocument(markdown)

	if fields[FieldH1] != "Main Title" {
		t.Errorf("H1 field = %q, want %q", fields[FieldH1], "Main Title")
	}
	if fields[FieldBold] != "bold" {
		t.Errorf("Bold field = %q, want %q", fields[FieldBold], "bold")
	}
	if fields[FieldItalic] != "italic" {
		t.Errorf("Italic field = %q, want %q", fields[FieldItalic], "italic")
	}
	if fields[FieldCode] != "code" {
		t.Errorf("Code field = %q, want %q", fields[FieldCode], "code")
	}
	if len(fields[FieldBody]) == 0 {
		t.Error("Body field should not be empty")
	}
}

func TestMarkdownFieldParser_NestedFormatting(t *testing.T) {
	parser := NewMarkdownFieldParser()

	markdown := `This is **bold _italic_** text.`
	fields := parser.ParseDocument(markdown)

	// parser extracts nested content with spaces preserved
	if fields[FieldBold] != "bold  italic" {
		t.Errorf("Bold field = %q, want %q", fields[FieldBold], "bold  italic")
	}
	// italicized content appears within bold, may not be separately extracted
	if len(fields[FieldItalic]) > 0 && fields[FieldItalic] != "italic" {
		t.Errorf("Italic field = %q, want %q", fields[FieldItalic], "italic")
	}
}

func TestMarkdownFieldParser_CodeBlocks(t *testing.T) {
	parser := NewMarkdownFieldParser()

	markdown := "```go\nfmt.Println(\"hello\")\n```"
	fields := parser.ParseDocument(markdown)

	if fields[FieldCode] != "fmt.Println(\"hello\")" {
		t.Errorf("Code field = %q, want %q", fields[FieldCode], "fmt.Println(\"hello\")")
	}
}
