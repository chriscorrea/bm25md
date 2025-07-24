package bm25md

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// MarkdownFieldParser extracts content from markdown documents
type MarkdownFieldParser struct {
	parser parser.Parser
}

// NewMarkdownFieldParser creates new AST-based parser instance
func NewMarkdownFieldParser() *MarkdownFieldParser {
	return &MarkdownFieldParser{
		parser: goldmark.DefaultParser(),
	}
}

// ParseDocument extracts field-specific content using AST traversal
func (p *MarkdownFieldParser) ParseDocument(content string) map[Field]string {
	// Initialize all fields with empty strings
	fields := make(map[Field]string)
	for field := range DefaultFieldWeights {
		fields[field] = ""
	}

	// parse markdown to AST
	source := []byte(content)
	reader := text.NewReader(source)
	doc := p.parser.Parse(reader)

	// storage for collected text by field type
	fieldTexts := make(map[Field][]string)
	for field := range DefaultFieldWeights {
		fieldTexts[field] = make([]string, 0)
	}

	// walk the AST and extract text based on node type
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.Heading:
			// extract header text based on level
			text := p.extractTextFromChildren(n, source)
			if text != "" {
				field := p.getHeaderField(n.Level)
				fieldTexts[field] = append(fieldTexts[field], text)
			}
			// skip children
			return ast.WalkSkipChildren, nil

		case *ast.CodeSpan:
			// extract inline code
			text := p.extractTextFromChildren(n, source)
			if text != "" {
				fieldTexts[FieldCode] = append(fieldTexts[FieldCode], text)
			}
			// skip children
			return ast.WalkSkipChildren, nil

		case *ast.FencedCodeBlock:
			// extract fenced code block content
			text := p.extractCodeBlockText(n, source)
			if text != "" {
				fieldTexts[FieldCode] = append(fieldTexts[FieldCode], text)
			}
			// skip children
			return ast.WalkSkipChildren, nil

		case *ast.CodeBlock:
			// extract indented code block content
			text := p.extractCodeBlockText(n, source)
			if text != "" {
				fieldTexts[FieldCode] = append(fieldTexts[FieldCode], text)
			}
			// Skip children as we've already processed them
			return ast.WalkSkipChildren, nil

		case *ast.Text:
			// only extract text if it's not inside a special element
			if !p.isInsideSpecialElement(node) {
				text := strings.TrimSpace(string(n.Segment.Value(source)))
				if text != "" {
					fieldTexts[FieldBody] = append(fieldTexts[FieldBody], text)
				}
			}

		default:
			// handle emphasis by checking Kind()
			if node.Kind() == ast.KindEmphasis {
				// check if it's strong (bold) or emphasis (italic)
				if n, ok := node.(*ast.Emphasis); ok {
					text := p.extractTextFromChildren(n, source)
					if text != "" {
						if n.Level == 2 { // ** or __
							fieldTexts[FieldBold] = append(fieldTexts[FieldBold], text)
						} else if n.Level == 1 { // * or _
							fieldTexts[FieldItalic] = append(fieldTexts[FieldItalic], text)
						}
					}
					// skip children
					return ast.WalkSkipChildren, nil
				}
			}
		}

		return ast.WalkContinue, nil
	})

	if err != nil {
		// if there's an error, fall back to original content in body
		fields[FieldBody] = content
		return fields
	}

	// join collected texts for each field
	for field, texts := range fieldTexts {
		if len(texts) > 0 {
			fields[field] = strings.Join(texts, " ")
		}
	}

	return fields
}

// getHeaderField returns the appropriate field for a header level
func (p *MarkdownFieldParser) getHeaderField(level int) Field {
	switch level {
	case 1:
		return FieldH1
	case 2:
		return FieldH2
	case 3:
		return FieldH3
	case 4:
		return FieldH4
	case 5:
		return FieldH5
	case 6:
		return FieldH6
	default:
		return FieldH6 // fallback - just in case
	}
}

// extractTextFromChildren recursively extracts plain text from all child nodes
func (p *MarkdownFieldParser) extractTextFromChildren(node ast.Node, source []byte) string {
	var buf bytes.Buffer

	// Walk through all children to extract text
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		p.extractTextRecursive(child, source, &buf)
	}

	return strings.TrimSpace(buf.String())
}

// extractTextRecursive recursively extracts text from a node and its children
func (p *MarkdownFieldParser) extractTextRecursive(node ast.Node, source []byte, buf *bytes.Buffer) {
	switch n := node.(type) {
	case *ast.Text:
		// extract the actual text content
		text := n.Segment.Value(source)
		buf.Write(text)

	case *ast.String:
		// extract string content
		buf.WriteString(string(n.Value))

	default:
		// for other nodes, recursively process children
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			p.extractTextRecursive(child, source, buf)
		}
	}

	// add space between adjacent text nodes
	if node.NextSibling() != nil {
		buf.WriteString(" ")
	}
}

// extractCodeBlockText extracts text from code blocks, removing language identifiers
func (p *MarkdownFieldParser) extractCodeBlockText(node ast.Node, source []byte) string {
	var buf bytes.Buffer

	// for fenced code blocks, skip the language identifier
	if fenced, ok := node.(*ast.FencedCodeBlock); ok {
		// process each line of the code block
		for i := 0; i < fenced.Lines().Len(); i++ {
			line := fenced.Lines().At(i)
			text := line.Value(source)
			buf.Write(text)
		}
	} else {
		// for regular code blocks, extract all text
		p.extractTextRecursive(node, source, &buf)
	}

	// clean up
	result := buf.String()
	result = strings.TrimSpace(result)

	// remove common language identifiers from the start (eg ```go)
	lines := strings.Split(result, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		// if first line looks like a short language identifier, skip it
		if len(firstLine) < 12 && !strings.Contains(firstLine, " ") && len(lines) > 1 {
			result = strings.Join(lines[1:], "\n")
		}
	}

	return strings.TrimSpace(result)
}

// isInsideSpecialElement checks if a node is inside a heading, code, or emphasis element
func (p *MarkdownFieldParser) isInsideSpecialElement(node ast.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.(type) {
		case *ast.Heading, *ast.CodeSpan, *ast.FencedCodeBlock, *ast.CodeBlock:
			return true
		default:
			if parent.Kind() == ast.KindEmphasis {
				return true
			}
		}
		parent = parent.Parent()
	}
	return false
}

// ParseDocuments parses multiple markdown documents into BM25md Documents
func (p *MarkdownFieldParser) ParseDocuments(contents []string) []Document {
	documents := make([]Document, len(contents))

	for i, content := range contents {
		fields := p.ParseDocument(content)
		documents[i] = Document{
			ID:       i,
			Fields:   fields,
			Original: content,
		}
	}

	return documents
}
