package core

import (
	"bytes"
	"testing"
)

// TestComputeLineOffsetsWithCRLF tests line offset computation with CRLF endings
func TestComputeLineOffsetsWithCRLF(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedLines  int
		expectedGrep   int // What line number grep reports for last line content
	}{
		{
			name:          "LF endings",
			content:       "line1\nline2\nline3\n",
			expectedLines: 3,
			expectedGrep:  3,
		},
		{
			name:          "CRLF endings",
			content:       "line1\r\nline2\r\nline3\r\n",
			expectedLines: 3,
			expectedGrep:  3,
		},
		{
			name:          "No trailing newline",
			content:       "line1\nline2\nline3",
			expectedLines: 3,
			expectedGrep:  3,
		},
		{
			name:          "Mixed endings (common in git)",
			content:       "line1\r\nline2\nline3\r\n",
			expectedLines: 3,
			expectedGrep:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentBytes := []byte(tt.content)
			offsets := computeLineOffsets(contentBytes)

			t.Logf("Content: %q", tt.content)
			t.Logf("Content length: %d", len(contentBytes))
			t.Logf("LineOffsets: %v", offsets)
			t.Logf("Number of offsets: %d", len(offsets))

			// Count \n characters (what grep uses)
			newlineCount := bytes.Count(contentBytes, []byte("\n"))
			t.Logf("Newline count: %d", newlineCount)

			// Test line number calculation using bytesToLine logic
			// Find "line3" in content
			line3Pos := bytes.Index(contentBytes, []byte("line3"))
			if line3Pos != -1 {
				// bytesToLine logic: count newlines before position + 1
				newlinesBeforeLine3 := bytes.Count(contentBytes[:line3Pos], []byte("\n"))
				calculatedLine := newlinesBeforeLine3 + 1
				t.Logf("Line3 found at byte %d, newlines before: %d, calculated line: %d",
					line3Pos, newlinesBeforeLine3, calculatedLine)

				if calculatedLine != tt.expectedGrep {
					t.Errorf("bytesToLine would report line %d, but grep reports %d",
						calculatedLine, tt.expectedGrep)
				}
			}

			// Verify offset count matches line count
			if len(offsets) != tt.expectedLines {
				t.Errorf("Expected %d line offsets, got %d", tt.expectedLines, len(offsets))
			}
		})
	}
}

// TestAuthRustReproduction tests the exact auth.rs scenario
func TestAuthRustReproduction(t *testing.T) {
	// Simplified version of auth.rs structure with CRLF
	content := []byte("pub struct Auth {\r\n" +
		"    db: Database,\r\n" +
		"}\r\n" +
		"\r\n" +
		"impl Auth {\r\n" +
		"    pub fn verify(&self, user: &str, pass: &str) -> Result<(), String> {\r\n" +
		"        if !self.db.check(user, pass) {\r\n" +
		"            return Err(\"invalid credentials\".into());\r\n" + // This should be line 8
		"        }\r\n" +
		"        Ok(())\r\n" +
		"    }\r\n" +
		"}\r\n")

	offsets := computeLineOffsets(content)
	t.Logf("LineOffsets: %v", offsets)
	t.Logf("Number of lines (offsets): %d", len(offsets))

	// Find "invalid credentials"
	pattern := []byte("invalid credentials")
	matchPos := bytes.Index(content, pattern)
	if matchPos == -1 {
		t.Fatal("Pattern not found")
	}

	// Calculate line number using bytesToLine logic
	newlinesBeforeMatch := bytes.Count(content[:matchPos], []byte("\n"))
	calculatedLine := newlinesBeforeMatch + 1

	t.Logf("Match at byte offset: %d", matchPos)
	t.Logf("Newlines before match: %d", newlinesBeforeMatch)
	t.Logf("Calculated line number: %d", calculatedLine)
	t.Logf("Expected line number: 8")

	if calculatedLine != 8 {
		t.Errorf("bytesToLine reports line %d, expected line 8", calculatedLine)
	}
}
