package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/standardbeagle/lci/internal/types"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ASTContentFilter provides AST-based content filtering for searches
type ASTContentFilter struct {
	astStore *ASTStore
}

// NewASTContentFilter creates a new AST-based content filter
func NewASTContentFilter(astStore *ASTStore) *ASTContentFilter {
	return &ASTContentFilter{
		astStore: astStore,
	}
}

// ContentType represents the type of content to search
type ContentType int

const (
	ContentAll ContentType = iota
	ContentCodeOnly
	ContentCommentsOnly
	ContentStringsOnly
	ContentTemplateStringsOnly
)

// TemplateStringInfo contains information about a template literal
type TemplateStringInfo struct {
	Tag      string // The tag (sql, gql, etc.) for tagged templates
	Content  string // The actual content
	FileID   types.FileID
	StartPos int
	EndPos   int
	Line     int
}

// ExtractContentByType extracts specific content types from a file using its AST
func (acf *ASTContentFilter) ExtractContentByType(fileID types.FileID, content []byte, contentType ContentType) ([]ContentRange, error) {
	tree, _, _, _, exists := acf.astStore.GetAST(fileID)
	if !exists || tree == nil {
		return nil, fmt.Errorf("no AST available for file %d", fileID)
	}

	root := tree.RootNode()
	if root == nil {
		return nil, errors.New("no root node in AST")
	}

	var ranges []ContentRange

	switch contentType {
	case ContentAll:
		// Return entire file content
		ranges = append(ranges, ContentRange{
			Start:   0,
			End:     len(content),
			Content: string(content),
			Type:    "all",
		})

	case ContentCommentsOnly:
		ranges = acf.extractComments(root, content)

	case ContentStringsOnly:
		ranges = acf.extractStringLiterals(root, content, false)

	case ContentTemplateStringsOnly:
		ranges = acf.extractStringLiterals(root, content, true)

	case ContentCodeOnly:
		// Extract everything except comments and strings
		commentRanges := acf.extractComments(root, content)
		stringRanges := acf.extractStringLiterals(root, content, false)
		ranges = acf.invertRanges(content, append(commentRanges, stringRanges...))
	}

	return ranges, nil
}

// ContentRange represents a range of content to search
type ContentRange struct {
	Start   int
	End     int
	Content string
	Type    string // "comment", "string", "template", "code", etc.
	Tag     string // For tagged templates: sql, gql, etc.
}

// extractComments extracts all comments from the AST
func (acf *ASTContentFilter) extractComments(node *sitter.Node, content []byte) []ContentRange {
	var ranges []ContentRange

	acf.traverseNode(node, func(n *sitter.Node) bool {
		kind := n.Kind()

		// Common comment node types across languages
		if kind == "comment" || kind == "line_comment" || kind == "block_comment" ||
			kind == "documentation_comment" || kind == "pragma" {
			ranges = append(ranges, ContentRange{
				Start:   int(n.StartByte()),
				End:     int(n.EndByte()),
				Content: string(content[n.StartByte():n.EndByte()]),
				Type:    "comment",
			})
			return false // Don't traverse into comments
		}

		return true // Continue traversal
	})

	return ranges
}

// extractStringLiterals extracts string literals, optionally including only template literals
func (acf *ASTContentFilter) extractStringLiterals(node *sitter.Node, content []byte, templatesOnly bool) []ContentRange {
	var ranges []ContentRange

	acf.traverseNode(node, func(n *sitter.Node) bool {
		kind := n.Kind()

		// JavaScript/TypeScript template literals
		if kind == "template_string" || kind == "template_literal" {
			ranges = append(ranges, acf.extractTemplateLiteral(n, content)...)
			return false // Don't traverse into the literal
		}

		// Tagged template expressions (sql`...`, gql`...`)
		if kind == "call_expression" {
			if tag := acf.extractTaggedTemplate(n, content); tag != nil {
				ranges = append(ranges, *tag)
				return false
			}
		}

		// Regular string literals (unless we only want templates)
		if !templatesOnly {
			if kind == "string" || kind == "string_literal" ||
				kind == "interpreted_string_literal" || kind == "raw_string_literal" {
				ranges = append(ranges, ContentRange{
					Start:   int(n.StartByte()),
					End:     int(n.EndByte()),
					Content: acf.getStringContent(n, content),
					Type:    "string",
				})
				return false // Don't traverse into the literal
			}
		}

		return true // Continue traversal
	})

	return ranges
}

// extractTemplateLiteral extracts content from a template literal
func (acf *ASTContentFilter) extractTemplateLiteral(node *sitter.Node, content []byte) []ContentRange {
	var ranges []ContentRange

	// Check if this is a tagged template
	parent := node.Parent()
	var tag string
	if parent != nil && parent.Kind() == "call_expression" {
		// Look for the tag (function name before the template)
		for i := uint(0); i < parent.ChildCount(); i++ {
			child := parent.Child(i)
			if child != nil && child.Kind() == "identifier" {
				tag = string(content[child.StartByte():child.EndByte()])
				break
			}
		}
	}

	// Extract the template content (excluding backticks and interpolations)
	var contentParts []string
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		kind := child.Kind()
		if kind == "template_string_content" || kind == "string_fragment" {
			contentParts = append(contentParts, string(content[child.StartByte():child.EndByte()]))
		}
		// Skip template_substitution nodes (${...})
	}

	fullContent := strings.Join(contentParts, "")
	ranges = append(ranges, ContentRange{
		Start:   int(node.StartByte()),
		End:     int(node.EndByte()),
		Content: fullContent,
		Type:    "template",
		Tag:     tag,
	})

	return ranges
}

// extractTaggedTemplate checks if a call expression is a tagged template
func (acf *ASTContentFilter) extractTaggedTemplate(node *sitter.Node, content []byte) *ContentRange {
	// Look for patterns like: sql`...`, gql`...`, etc.
	var tag string
	var templateNode *sitter.Node

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		kind := child.Kind()

		// Get the function/tag name
		if kind == "identifier" {
			potentialTag := string(content[child.StartByte():child.EndByte()])
			// Check if it's a known template tag
			if acf.isTemplateTag(potentialTag) {
				tag = potentialTag
			}
		}

		// Get the template literal
		if kind == "template_string" || kind == "template_literal" {
			templateNode = child
		}
	}

	if tag != "" && templateNode != nil {
		// Extract template content
		ranges := acf.extractTemplateLiteral(templateNode, content)
		if len(ranges) > 0 {
			result := ranges[0]
			result.Tag = tag
			return &result
		}
	}

	return nil
}

// isTemplateTag checks if a string is a known template tag
func (acf *ASTContentFilter) isTemplateTag(tag string) bool {
	knownTags := []string{
		"sql", "SQL",
		"gql", "graphql", "GraphQL",
		"html", "HTML",
		"css", "CSS",
		"md", "markdown",
		"safeHtml", "safeSQL",
		"query", "mutation", "fragment", // GraphQL specific
	}

	for _, known := range knownTags {
		if tag == known {
			return true
		}
	}

	return false
}

// getStringContent extracts the actual string content (without quotes)
func (acf *ASTContentFilter) getStringContent(node *sitter.Node, content []byte) string {
	text := string(content[node.StartByte():node.EndByte()])

	// Remove quotes
	if len(text) >= 2 {
		firstChar := text[0]
		lastChar := text[len(text)-1]

		if (firstChar == '"' && lastChar == '"') ||
			(firstChar == '\'' && lastChar == '\'') ||
			(firstChar == '`' && lastChar == '`') {
			return text[1 : len(text)-1]
		}
	}

	return text
}

// invertRanges creates ranges for everything except the given ranges
func (acf *ASTContentFilter) invertRanges(content []byte, excludeRanges []ContentRange) []ContentRange {
	if len(excludeRanges) == 0 {
		return []ContentRange{{
			Start:   0,
			End:     len(content),
			Content: string(content),
			Type:    "code",
		}}
	}

	// Sort ranges by start position
	// (In production, use a proper sort)
	var result []ContentRange
	lastEnd := 0

	for _, r := range excludeRanges {
		if r.Start > lastEnd {
			result = append(result, ContentRange{
				Start:   lastEnd,
				End:     r.Start,
				Content: string(content[lastEnd:r.Start]),
				Type:    "code",
			})
		}
		if r.End > lastEnd {
			lastEnd = r.End
		}
	}

	// Add final range if needed
	if lastEnd < len(content) {
		result = append(result, ContentRange{
			Start:   lastEnd,
			End:     len(content),
			Content: string(content[lastEnd:]),
			Type:    "code",
		})
	}

	return result
}

// traverseNode recursively traverses the AST
func (acf *ASTContentFilter) traverseNode(node *sitter.Node, visitor func(*sitter.Node) bool) {
	if node == nil {
		return
	}

	if !visitor(node) {
		return // Visitor returned false, stop traversing this branch
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			acf.traverseNode(child, visitor)
		}
	}
}

// ExtractSQLQueries finds SQL queries in template strings using AST
func (acf *ASTContentFilter) ExtractSQLQueries(fileID types.FileID, content []byte) ([]TemplateStringInfo, error) {
	ranges, err := acf.ExtractContentByType(fileID, content, ContentTemplateStringsOnly)
	if err != nil {
		return nil, err
	}

	var results []TemplateStringInfo
	for _, r := range ranges {
		// Check if content looks like SQL
		if acf.looksLikeSQL(r.Content) || r.Tag == "sql" || r.Tag == "SQL" {
			results = append(results, TemplateStringInfo{
				Tag:      r.Tag,
				Content:  r.Content,
				FileID:   fileID,
				StartPos: r.Start,
				EndPos:   r.End,
				Line:     acf.getLineNumber(content, r.Start),
			})
		}
	}

	return results, nil
}

// ExtractGraphQLQueries finds GraphQL queries in template strings using AST
func (acf *ASTContentFilter) ExtractGraphQLQueries(fileID types.FileID, content []byte) ([]TemplateStringInfo, error) {
	ranges, err := acf.ExtractContentByType(fileID, content, ContentTemplateStringsOnly)
	if err != nil {
		return nil, err
	}

	var results []TemplateStringInfo
	for _, r := range ranges {
		// Check if content looks like GraphQL
		if acf.looksLikeGraphQL(r.Content) || r.Tag == "gql" || r.Tag == "graphql" || r.Tag == "GraphQL" {
			results = append(results, TemplateStringInfo{
				Tag:      r.Tag,
				Content:  r.Content,
				FileID:   fileID,
				StartPos: r.Start,
				EndPos:   r.End,
				Line:     acf.getLineNumber(content, r.Start),
			})
		}
	}

	return results, nil
}

// looksLikeSQL checks if content appears to be SQL
func (acf *ASTContentFilter) looksLikeSQL(content string) bool {
	upperContent := strings.ToUpper(content)
	sqlKeywords := []string{"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "FROM ", "WHERE ", "JOIN "}

	for _, keyword := range sqlKeywords {
		if strings.Contains(upperContent, keyword) {
			return true
		}
	}

	return false
}

// looksLikeGraphQL checks if content appears to be GraphQL
func (acf *ASTContentFilter) looksLikeGraphQL(content string) bool {
	lowerContent := strings.ToLower(content)
	gqlPatterns := []string{"query ", "mutation ", "subscription ", "fragment ", "query{", "mutation{"}

	for _, pattern := range gqlPatterns {
		if strings.Contains(lowerContent, pattern) {
			return true
		}
	}

	return false
}

// getLineNumber calculates the line number for a given byte position
func (acf *ASTContentFilter) getLineNumber(content []byte, pos int) int {
	line := 1
	for i := 0; i < pos && i < len(content); i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}
