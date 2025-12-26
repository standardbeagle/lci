package core

import (
	"regexp"
	"strings"
)

// ContentFilter defines what content types to include/exclude in search
type ContentFilter struct {
	CommentsOnly    bool // Search only in comments
	CodeOnly        bool // Search only in code (exclude comments and strings)
	StringsOnly     bool // Search only in string literals
	TemplateStrings bool // Include template strings (sql``, gql``, etc.)
	ExcludeComments bool // Exclude comments from search (legacy, kept for compatibility)
}

// TaggedTemplateExtractor extracts SQL, GraphQL, and other tagged template literals
type TaggedTemplateExtractor struct {
	// Common template tags to look for
	templateTags []string
}

// NewTaggedTemplateExtractor creates an extractor for common template literals
func NewTaggedTemplateExtractor() *TaggedTemplateExtractor {
	return &TaggedTemplateExtractor{
		templateTags: []string{
			"sql", "SQL",
			"gql", "graphql", "GraphQL",
			"html", "HTML",
			"css", "CSS",
			"md", "markdown",
			"safeHtml", "safeSQL", // Common safe template tags
		},
	}
}

// ExtractTemplateStrings finds tagged template literals in JavaScript/TypeScript code
func (tte *TaggedTemplateExtractor) ExtractTemplateStrings(content string) []TemplateString {
	var results []TemplateString

	// Regex for tagged templates: tag`content`
	// Handles multi-line and nested backticks
	// But exclude templates that are part of function calls (handled separately)
	for _, tag := range tte.templateTags {
		pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(tag) + "`([^`]*(?:``[^`]*)*)`")
		matches := pattern.FindAllStringSubmatchIndex(content, -1)

		for _, match := range matches {
			if len(match) >= 4 {
				// Check that this isn't part of a function call by looking backwards
				isPartOfFunction := false
				if match[0] > 0 {
					// Look backwards for pattern like "word(" before our tag
					beforeText := content[:match[0]]
					funcCallPattern := regexp.MustCompile(`\w+\s*\(\s*$`)
					if funcCallPattern.MatchString(beforeText) {
						isPartOfFunction = true
					}
				}

				if !isPartOfFunction {
					results = append(results, TemplateString{
						Tag:        tag,
						Content:    content[match[2]:match[3]],
						StartPos:   match[0],
						EndPos:     match[1],
						LineNumber: countNewlines(content[:match[0]]) + 1,
					})
				}
			}
		}
	}

	// Also find template literals with function calls: db.query(SQL`...`)
	funcPattern := regexp.MustCompile(`(?s)\b(\w+)\s*\(\s*(sql|SQL|gql|graphql|GraphQL)` + "`([^`]*(?:``[^`]*)*)`")
	funcMatches := funcPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range funcMatches {
		if len(match) >= 8 {
			results = append(results, TemplateString{
				Tag:        content[match[4]:match[5]], // The template tag
				Content:    content[match[6]:match[7]], // The template content
				Function:   content[match[2]:match[3]], // The function name
				StartPos:   match[0],
				EndPos:     match[1],
				LineNumber: countNewlines(content[:match[0]]) + 1,
			})
		}
	}

	return results
}

// TemplateString represents an extracted template literal
type TemplateString struct {
	Tag        string // sql, gql, etc.
	Content    string // The actual SQL/GraphQL query
	Function   string // Optional: function it's passed to
	StartPos   int
	EndPos     int
	LineNumber int
}

// FilterContent applies content filtering based on the filter settings
func FilterContent(content string, filter ContentFilter) []ContentSegment {
	segments := []ContentSegment{}

	if filter.CommentsOnly {
		// Extract only comments
		segments = append(segments, extractComments(content)...)
	} else if filter.CodeOnly {
		// Extract code, excluding comments and string literals
		segments = append(segments, extractCodeOnly(content)...)
	} else if filter.StringsOnly {
		// Extract only string literals
		segments = append(segments, extractStringLiterals(content)...)

		if filter.TemplateStrings {
			// Also include template strings
			extractor := NewTaggedTemplateExtractor()
			templates := extractor.ExtractTemplateStrings(content)
			for _, tmpl := range templates {
				segments = append(segments, ContentSegment{
					Type:    "template_string",
					Content: tmpl.Content,
					Start:   tmpl.StartPos,
					End:     tmpl.EndPos,
					Tag:     tmpl.Tag,
				})
			}
		}
	} else {
		// Default: include everything except what's explicitly excluded
		if filter.ExcludeComments {
			segments = append(segments, extractNonComments(content)...)
		} else {
			// Include everything
			segments = append(segments, ContentSegment{
				Type:    "all",
				Content: content,
				Start:   0,
				End:     len(content),
			})
		}
	}

	return segments
}

// ContentSegment represents a segment of content to search
type ContentSegment struct {
	Type    string // "comment", "code", "string", "template_string", "all"
	Content string
	Start   int
	End     int
	Tag     string // For template strings: sql, gql, etc.
}

// Helper functions for content extraction

func extractComments(content string) []ContentSegment {
	var segments []ContentSegment

	// Single-line comments
	singleLinePattern := regexp.MustCompile(`//[^\n]*`)
	matches := singleLinePattern.FindAllStringIndex(content, -1)
	for _, match := range matches {
		segments = append(segments, ContentSegment{
			Type:    "comment",
			Content: content[match[0]:match[1]],
			Start:   match[0],
			End:     match[1],
		})
	}

	// Multi-line comments
	multiLinePattern := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	matches = multiLinePattern.FindAllStringIndex(content, -1)
	for _, match := range matches {
		segments = append(segments, ContentSegment{
			Type:    "comment",
			Content: content[match[0]:match[1]],
			Start:   match[0],
			End:     match[1],
		})
	}

	// Language-specific comments (Python, Ruby, Shell)
	hashPattern := regexp.MustCompile(`#[^\n]*`)
	matches = hashPattern.FindAllStringIndex(content, -1)
	for _, match := range matches {
		segments = append(segments, ContentSegment{
			Type:    "comment",
			Content: content[match[0]:match[1]],
			Start:   match[0],
			End:     match[1],
		})
	}

	return segments
}

func extractStringLiterals(content string) []ContentSegment {
	var segments []ContentSegment

	// Double-quoted strings
	doubleQuotePattern := regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	matches := doubleQuotePattern.FindAllStringIndex(content, -1)
	for _, match := range matches {
		segments = append(segments, ContentSegment{
			Type:    "string",
			Content: content[match[0]+1 : match[1]-1], // Remove quotes
			Start:   match[0],
			End:     match[1],
		})
	}

	// Single-quoted strings
	singleQuotePattern := regexp.MustCompile(`'(?:[^'\\]|\\.)*'`)
	matches = singleQuotePattern.FindAllStringIndex(content, -1)
	for _, match := range matches {
		segments = append(segments, ContentSegment{
			Type:    "string",
			Content: content[match[0]+1 : match[1]-1], // Remove quotes
			Start:   match[0],
			End:     match[1],
		})
	}

	// Template literals (backticks) - but not tagged ones
	templatePattern := regexp.MustCompile("`(?:[^`\\\\]|\\\\.)*`")
	matches = templatePattern.FindAllStringIndex(content, -1)
	for _, match := range matches {
		// Skip if this backtick is preceded by a word character (tagged template)
		if match[0] > 0 {
			prevChar := content[match[0]-1]
			if (prevChar >= 'a' && prevChar <= 'z') || (prevChar >= 'A' && prevChar <= 'Z') ||
				(prevChar >= '0' && prevChar <= '9') || prevChar == '_' {
				continue // Skip tagged template literals
			}
		}

		segments = append(segments, ContentSegment{
			Type:    "string",
			Content: content[match[0]+1 : match[1]-1], // Remove backticks
			Start:   match[0],
			End:     match[1],
		})
	}

	return segments
}

func extractCodeOnly(content string) []ContentSegment {
	// This would need a more sophisticated parser to properly exclude
	// comments and strings while keeping code.
	// For now, a simplified implementation:

	// Remove comments
	noComments := removeComments(content)
	// Remove string literals
	noStrings := removeStringLiterals(noComments)

	return []ContentSegment{{
		Type:    "code",
		Content: noStrings,
		Start:   0,
		End:     len(noStrings),
	}}
}

func extractNonComments(content string) []ContentSegment {
	noComments := removeComments(content)
	return []ContentSegment{{
		Type:    "code_and_strings",
		Content: noComments,
		Start:   0,
		End:     len(noComments),
	}}
}

func removeComments(content string) string {
	// Remove single-line comments
	content = regexp.MustCompile(`//[^\n]*`).ReplaceAllString(content, "")
	// Remove multi-line comments
	content = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(content, "")
	// Remove hash comments
	content = regexp.MustCompile(`#[^\n]*`).ReplaceAllString(content, "")
	return content
}

func removeStringLiterals(content string) string {
	// Remove double-quoted strings
	content = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`).ReplaceAllString(content, `""`)
	// Remove single-quoted strings
	content = regexp.MustCompile(`'(?:[^'\\]|\\.)*'`).ReplaceAllString(content, `''`)
	// Remove template literals
	content = regexp.MustCompile("`(?:[^`\\\\]|\\\\.)*`").ReplaceAllString(content, "``")
	return content
}

func countNewlines(s string) int {
	return strings.Count(s, "\n")
}

// IsTemplateStringQuery checks if a search pattern is likely a SQL or GraphQL query
func IsTemplateStringQuery(pattern string) bool {
	upperPattern := strings.ToUpper(pattern)

	// SQL patterns
	sqlKeywords := []string{"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "FROM ", "WHERE ",
		"JOIN ", "CREATE TABLE", "ALTER TABLE", "DROP TABLE"}
	for _, keyword := range sqlKeywords {
		if strings.Contains(upperPattern, keyword) {
			return true
		}
	}

	// GraphQL patterns
	gqlKeywords := []string{"query ", "mutation ", "subscription ", "fragment ",
		"query {", "mutation {", "subscription {"}
	lowerPattern := strings.ToLower(pattern)
	for _, keyword := range gqlKeywords {
		if strings.Contains(lowerPattern, keyword) {
			return true
		}
	}

	return false
}
