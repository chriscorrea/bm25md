package bm25md

import (
	"strings"
	"testing"
)

func TestMarkdownFieldParser_ParseDocument(t *testing.T) {
	parser := NewMarkdownFieldParser()

	tests := []struct {
		name     string
		input    string
		expected map[Field]string
	}{
		{
			name: "headers",
			input: `# Main Title
## Subtitle
### Section
#### Subsection
##### Small Header
###### Tiny Header
Body text here`,
			expected: map[Field]string{
				FieldH1:   "Main Title",
				FieldH2:   "Subtitle",
				FieldH3:   "Section",
				FieldH4:   "Subsection",
				FieldH5:   "Small Header",
				FieldH6:   "Tiny Header",
				FieldBody: "Body text here",
			},
		},
		{
			name: "formatting",
			input: `Normal text with **bold text** and *italic text*.
Also __bold__ and _italic_ alternatives.`,
			expected: map[Field]string{
				FieldBold:   "bold text bold",
				FieldItalic: "italic text italic",
				FieldBody:   "Normal text with and . Also and alternatives.",
			},
		},
		{
			name:  "code",
			input: "Text with `inline code` and:\n```python\ndef hello():\n    print('world')\n```\nMore text",
			expected: map[Field]string{
				FieldCode: "inline code def hello(): print('world')",
				FieldBody: "Text with and: More text",
			},
		},
		{
			name:  "mixed content",
			input: "# Baking Guide\n## Sifting Flour\nAlways **sift** your flour for *better* cakes.\nUse `350°F` for most recipes.",
			expected: map[Field]string{
				FieldH1:     "Baking Guide",
				FieldH2:     "Sifting Flour",
				FieldBold:   "sift",
				FieldItalic: "better",
				FieldCode:   "350°F",
				FieldBody:   "Always your flour for cakes. Use for most recipes.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseDocument(tt.input)

			// check each expected field
			for field, expectedContent := range tt.expected {
				// normalize whitespace for comparison
				got := normalizeWhitespace(result[field])
				want := normalizeWhitespace(expectedContent)

				if got != want {
					t.Errorf("Field %s = %q, want %q", field, got, want)
				}
			}
		})
	}
}

func TestMarkdownFieldParser_CodeBlockExtraction(t *testing.T) {
	parser := NewMarkdownFieldParser()

	tests := []struct {
		name         string
		input        string
		expectedCode string
		expectedBody string
	}{
		{
			name:         "simple code block",
			input:        "Before code\n```\ncode here\n```\nAfter code",
			expectedCode: "code here",
			expectedBody: "Before code After code",
		},
		{
			name:         "code block with language",
			input:        "Text\n```go\nfunc main() {}\n```\nMore",
			expectedCode: "func main() {}",
			expectedBody: "Text More",
		},
		{
			name:         "multiple code blocks",
			input:        "Start\n```\nfirst\n```\nMiddle\n```\nsecond\n```\nEnd",
			expectedCode: "first second",
			expectedBody: "Start Middle End",
		},
		{
			name:         "inline and block code",
			input:        "Use `inline` code\n```\nblock code\n```\nDone",
			expectedCode: "inline block code",
			expectedBody: "Use code Done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseDocument(tt.input)

			gotCode := normalizeWhitespace(result[FieldCode])
			wantCode := normalizeWhitespace(tt.expectedCode)
			if gotCode != wantCode {
				t.Errorf("Code field = %q, want %q", gotCode, wantCode)
			}

			gotBody := normalizeWhitespace(result[FieldBody])
			wantBody := normalizeWhitespace(tt.expectedBody)
			if gotBody != wantBody {
				t.Errorf("Body field = %q, want %q", gotBody, wantBody)
			}
		})
	}
}

func TestMarkdownFieldParser_BodyCleaning(t *testing.T) {
	parser := NewMarkdownFieldParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "list markers",
			input:    "- Item one\n* Item two\n+ Item three\n1. Numbered",
			expected: "Item one Item two Item three Numbered",
		},
		{
			name:     "blockquotes",
			input:    "> Quote line one\n> Quote line two",
			expected: "Quote line one Quote line two",
		},
		{
			name:     "links",
			input:    "Check [this link](http://example.com) out",
			expected: "Check this link out",
		},
		{
			name:     "images",
			input:    "![alt text](image.png) caption",
			expected: "alt text caption",
		},
		{
			name:     "horizontal rules",
			input:    "Text\n\n---\n\nMore text",
			expected: "Text More text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseDocument(tt.input)
			got := normalizeWhitespace(result[FieldBody])
			want := normalizeWhitespace(tt.expected)

			if got != want {
				t.Errorf("Body = %q, want %q", got, want)
			}
		})
	}
}

func TestMarkdownFieldParser_ParseDocuments(t *testing.T) {
	parser := NewMarkdownFieldParser()

	contents := []string{
		"# Doc 1\nFirst document",
		"# Doc 2\nSecond document",
		"# Doc 3\nThird document",
	}

	docs := parser.ParseDocuments(contents)

	if len(docs) != 3 {
		t.Errorf("ParseDocuments returned %d documents, want 3", len(docs))
	}

	for i, doc := range docs {
		if doc.ID != i {
			t.Errorf("Document %d has ID %d, want %d", i, doc.ID, i)
		}

		if doc.Original != contents[i] {
			t.Errorf("Document %d original content mismatch", i)
		}

		expectedH1 := strings.TrimPrefix(strings.Split(contents[i], "\n")[0], "# ")
		if doc.Fields[FieldH1] != expectedH1 {
			t.Errorf("Document %d H1 = %q, want %q", i, doc.Fields[FieldH1], expectedH1)
		}
	}
}

func TestMarkdownFieldParser_ComplexDocument(t *testing.T) {
	parser := NewMarkdownFieldParser()

	// test with a markdown recipe doc
	input := `# Carrot Cake Recipe

## Ingredients

For the **cake**:
- 2 cups *sifted* flour
- 2 tsp baking soda
- 1/2 tsp salt

### Preparation

1. Preheat oven to ` + "`350°F`" + `
2. **Sift** flour and mix dry ingredients
3. Combine wet ingredients separately

## Instructions

Mix everything together and bake for *32 minutes*.

### Tips

> Always use fresh carrots for best results

Check doneness by inserting a sword.

---

**Note**: This recipe serves 1 person, me.`

	result := parser.ParseDocument(input)

	// verify key content is extracted to correct fields
	if !strings.Contains(result[FieldH1], "Carrot Cake Recipe") {
		t.Error("H1 should contain 'Carrot Cake Recipe'")
	}

	if !strings.Contains(result[FieldH2], "Ingredients") {
		t.Error("H2 should contain 'Ingredients'")
	}

	if !strings.Contains(result[FieldH2], "Instructions") {
		t.Error("H2 should contain 'Instructions'")
	}

	if !strings.Contains(result[FieldBold], "cake") {
		t.Error("Bold should contain 'cake'")
	}

	if !strings.Contains(result[FieldBold], "Sift") {
		t.Error("Bold should contain 'Sift'")
	}

	if !strings.Contains(result[FieldItalic], "sifted") {
		t.Error("Italic should contain 'sifted'")
	}

	if !strings.Contains(result[FieldCode], "350°F") {
		t.Error("Code should contain '350°F'")
	}

	if !strings.Contains(result[FieldBody], "use fresh carrots") {
		t.Error("Body should contain blockquote content")
	}
}

// normalizeWhitespace helps with test comparisons by normalizing whitespace
func normalizeWhitespace(s string) string {
	// replace multiple spaces with single space
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}
