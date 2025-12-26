package search

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

func TestLineNumberCalculation(t *testing.T) {
	// Test content matching the Rust file
	content := []byte(`use std::error::Error;
use std::time::{SystemTime, Duration};

/// Token represents an authentication token
pub struct Token {
    pub value: String,
    pub expires_at: SystemTime,
}

/// AuthService handles authentication
pub struct AuthService {
    user_service: Box<dyn std::any::Any>,
}

impl AuthService {
    /// Creates a new AuthService
    pub fn new(user_service: Box<dyn std::any::Any>) -> Self {
        AuthService { user_service }
    }

    /// Authenticate authenticates a user
    pub fn authenticate(&self, username: &str, password: &str) -> Result<Token, Box<dyn Error>> {
        if username.is_empty() || password.is_empty() {
            return Err("invalid credentials".into());
        }

        let token = Token {
            value: "token-value".to_string(),
            expires_at: SystemTime::now() + Duration::from_secs(86400),
        };

        Ok(token)
    }

    /// ValidateToken validates an authentication token
    pub fn validate_token(&self, token: &str) -> Result<(), Box<dyn Error>> {
        if token.is_empty() {
            return Err("invalid token".into());
        }
        Ok(())
    }
}
`)

	// Compute line offsets
	lineOffsets := types.ComputeLineOffsets(content)

	// Log all line offsets
	t.Logf("Total lines: %d", len(lineOffsets))
	for i, offset := range lineOffsets {
		if i < len(lineOffsets) {
			t.Logf("LineOffsets[%d] = %d (line %d starts here)", i, offset, i+1)
		}
	}

	// Find where "invalid credentials" appears
	searchText := "invalid credentials"
	matchStart := -1
	for i := 0; i < len(content)-len(searchText); i++ {
		if string(content[i:i+len(searchText)]) == searchText {
			matchStart = i
			break
		}
	}

	if matchStart == -1 {
		t.Fatal("Could not find 'invalid credentials' in content")
	}

	t.Logf("Match starts at byte offset: %d", matchStart)

	// Count newlines before match to get actual line number
	newlineCount := 0
	for i := 0; i < matchStart; i++ {
		if content[i] == '\n' {
			newlineCount++
		}
	}
	actualLine := newlineCount + 1
	t.Logf("Actual line (newline count + 1): %d", actualLine)

	// Binary search for line using LineOffsets (current algorithm)
	l, r := 0, len(lineOffsets)-1
	for l <= r {
		m := (l + r) / 2
		if lineOffsets[m] <= matchStart {
			l = m + 1
		} else {
			r = m - 1
		}
	}
	calculatedLine := r + 1
	t.Logf("Calculated line (binary search, r+1): %d", calculatedLine)

	// Check if they match
	if calculatedLine != actualLine {
		t.Errorf("Line number mismatch! Binary search gives %d, actual is %d", calculatedLine, actualLine)
	}

	// Expected: line 24 (based on manual counting and grep output)
	if actualLine != 24 {
		t.Errorf("Actual line should be 24, but got %d", actualLine)
	}
}
