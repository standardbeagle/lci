package core

import (
	"strings"
	"testing"
)

// TestTaggedTemplateExtractor tests the tagged template extractor.
func TestTaggedTemplateExtractor(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
		validate func(t *testing.T, results []TemplateString)
	}{
		{
			name: "SQL template literal",
			content: `
const query = sql` + "`" + `
  SELECT * FROM users 
  WHERE id = ${userId}
` + "`" + `;`,
			expected: 1,
			validate: func(t *testing.T, results []TemplateString) {
				if results[0].Tag != "sql" {
					t.Errorf("Expected tag 'sql', got %s", results[0].Tag)
				}
				if !strings.Contains(results[0].Content, "SELECT * FROM users") {
					t.Errorf("SQL content not extracted properly")
				}
			},
		},
		{
			name: "GraphQL template literal",
			content: `
const GET_USER = gql` + "`" + `
  query GetUser($id: ID!) {
    user(id: $id) {
      name
      email
    }
  }
` + "`" + `;`,
			expected: 1,
			validate: func(t *testing.T, results []TemplateString) {
				if results[0].Tag != "gql" {
					t.Errorf("Expected tag 'gql', got %s", results[0].Tag)
				}
				if !strings.Contains(results[0].Content, "query GetUser") {
					t.Errorf("GraphQL content not extracted properly")
				}
			},
		},
		{
			name: "Function with SQL template",
			content: `
db.query(SQL` + "`" + `
  INSERT INTO products (name, price) 
  VALUES (${name}, ${price})
` + "`" + `);`,
			expected: 1,
			validate: func(t *testing.T, results []TemplateString) {
				if results[0].Tag != "SQL" {
					t.Errorf("Expected tag 'SQL', got %s", results[0].Tag)
				}
				if results[0].Function != "query" {
					t.Errorf("Expected function 'query', got %s", results[0].Function)
				}
				if !strings.Contains(results[0].Content, "INSERT INTO products") {
					t.Errorf("SQL content not extracted properly")
				}
			},
		},
		{
			name: "Multiple template strings",
			content: `
const q1 = sql` + "`SELECT * FROM users`" + `;
const q2 = gql` + "`query { users { id } }`" + `;
const html = html` + "`<div>Hello</div>`" + `;`,
			expected: 3,
			validate: func(t *testing.T, results []TemplateString) {
				tags := make(map[string]bool)
				for _, r := range results {
					tags[r.Tag] = true
				}
				if !tags["sql"] || !tags["gql"] || !tags["html"] {
					t.Errorf("Not all template types found")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewTaggedTemplateExtractor()
			results := extractor.ExtractTemplateStrings(tt.content)

			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
			}

			if tt.validate != nil {
				tt.validate(t, results)
			}
		})
	}
}

// TestContentFilter tests the content filter.
func TestContentFilter(t *testing.T) {
	testContent := `
// This is a comment
function test() {
  const message = "Hello, world!";
  const query = sql` + "`SELECT * FROM users`" + `;
  /* Multi-line
     comment */
  return message;
}
`

	tests := []struct {
		name     string
		filter   ContentFilter
		validate func(t *testing.T, segments []ContentSegment)
	}{
		{
			name: "Comments only",
			filter: ContentFilter{
				CommentsOnly: true,
			},
			validate: func(t *testing.T, segments []ContentSegment) {
				for _, seg := range segments {
					if seg.Type != "comment" {
						t.Errorf("Expected only comments, got type %s", seg.Type)
					}
				}
				// Should find both single and multi-line comments
				foundSingle := false
				foundMulti := false
				for _, seg := range segments {
					if strings.Contains(seg.Content, "This is a comment") {
						foundSingle = true
					}
					if strings.Contains(seg.Content, "Multi-line") {
						foundMulti = true
					}
				}
				if !foundSingle || !foundMulti {
					t.Error("Not all comments found")
				}
			},
		},
		{
			name: "Strings only",
			filter: ContentFilter{
				StringsOnly: true,
			},
			validate: func(t *testing.T, segments []ContentSegment) {
				foundHello := false
				for _, seg := range segments {
					if strings.Contains(seg.Content, "Hello, world!") {
						foundHello = true
					}
				}
				if !foundHello {
					t.Error("String literal not found")
				}
			},
		},
		{
			name: "Strings with templates",
			filter: ContentFilter{
				StringsOnly:     true,
				TemplateStrings: true,
			},
			validate: func(t *testing.T, segments []ContentSegment) {
				foundSQL := false
				for _, seg := range segments {
					if seg.Type == "template_string" && seg.Tag == "sql" {
						foundSQL = true
					}
				}
				if !foundSQL {
					t.Error("SQL template string not found")
				}
			},
		},
		{
			name: "Code only",
			filter: ContentFilter{
				CodeOnly: true,
			},
			validate: func(t *testing.T, segments []ContentSegment) {
				for _, seg := range segments {
					// Should not contain comments
					if strings.Contains(seg.Content, "This is a comment") {
						t.Error("Comments should be excluded in code-only mode")
					}
					if strings.Contains(seg.Content, "Multi-line") {
						t.Error("Multi-line comments should be excluded")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := FilterContent(testContent, tt.filter)
			if tt.validate != nil {
				tt.validate(t, segments)
			}
		})
	}
}

// TestIsTemplateStringQuery tests the is template string query.
func TestIsTemplateStringQuery(t *testing.T) {
	tests := []struct {
		pattern  string
		expected bool
	}{
		{"SELECT * FROM users WHERE id = 123", true},
		{"INSERT INTO products VALUES", true},
		{"UPDATE users SET name = 'test'", true},
		{"DELETE FROM orders", true},
		{"query { users { id name } }", true},
		{"mutation CreateUser($input: UserInput!)", true},
		{"fragment UserFields on User", true},
		{"just a regular string", false},
		{"this is not SQL or GraphQL", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern[:min(20, len(tt.pattern))], func(t *testing.T) {
			result := IsTemplateStringQuery(tt.pattern)
			if result != tt.expected {
				t.Errorf("Pattern %q: expected %v, got %v", tt.pattern, tt.expected, result)
			}
		})
	}
}

// min function already defined in reference_tracker.go
