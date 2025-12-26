package core

import (
	"sort"
	"testing"
)

// TestExtractHTMLFragments tests the extract h t m l fragments.
func TestExtractHTMLFragments(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected []string
	}{
		{
			name: "Simple div with class",
			html: `<div className="error-message">Error: User not found</div>`,
			expected: []string{
				"div", "className", "error", "message", "Error", "User", "not", "found",
			},
		},
		{
			name: "JSX with expressions",
			html: `<h1>{user.name}</h1><p>{user.bio}</p>`,
			expected: []string{
				"h1", "p", "user", "name", "bio", "user.name", "user.bio",
			},
		},
		{
			name: "React component with props",
			html: `<Button variant="primary" onClick={handleClick}>Submit</Button>`,
			expected: []string{
				"Button", "variant", "primary", "onClick", "handleClick", "Submit",
			},
		},
		{
			name: "Data attributes",
			html: `<div data-testid="error-modal" aria-label="Error message">Test</div>`,
			expected: []string{
				"div", "data-testid", "error", "modal", "aria-label", "Error", "message", "Test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragments := extractHTMLFragments(tt.html, 3)

			// Sort for consistent comparison
			sort.Strings(fragments)
			sort.Strings(tt.expected)

			t.Logf("HTML: %s", tt.html)
			t.Logf("Got fragments: %v", fragments)

			// Check coverage
			found := 0
			missing := []string{}
			for _, exp := range tt.expected {
				hasIt := false
				for _, frag := range fragments {
					if frag == exp {
						hasIt = true
						found++
						break
					}
				}
				if !hasIt {
					missing = append(missing, exp)
				}
			}

			coverage := float64(found) / float64(len(tt.expected)) * 100
			t.Logf("Coverage: %.1f%% (%d/%d expected fragments found)",
				coverage, found, len(tt.expected))

			if len(missing) > 0 {
				t.Logf("Missing: %v", missing)
			}

			if coverage < 80 {
				t.Errorf("Expected at least 80%% coverage, got %.1f%%", coverage)
			}
		})
	}
}

// TestEnhancedFragmentString tests the enhanced fragment string.
func TestEnhancedFragmentString(t *testing.T) {
	ase := &AssemblySearchEngine{
		minFragmentLength: 3,
	}

	tests := []struct {
		name        string
		input       string
		shouldMatch []string
		isHTML      bool
	}{
		{
			name:        "HTML content",
			input:       `<div className="user-card">Welcome {user.name}</div>`,
			shouldMatch: []string{"div", "className", "user", "card", "Welcome", "name"},
			isHTML:      true,
		},
		{
			name:        "Regular text",
			input:       "Error: Failed to connect to database",
			shouldMatch: []string{"Error", "Failed", "connect", "database"},
			isHTML:      false,
		},
		{
			name:        "Mixed JSX",
			input:       `<Button onClick={() => console.log('clicked')}>Click me</Button>`,
			shouldMatch: []string{"Button", "onClick", "console.log", "Click"},
			isHTML:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragments := ase.enhancedFragmentString(tt.input, 3)

			t.Logf("Input: %s", tt.input)
			t.Logf("Fragments: %v", fragments)

			for _, expected := range tt.shouldMatch {
				found := false
				for _, frag := range fragments {
					if frag == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find fragment '%s' but didn't", expected)
				}
			}
		})
	}
}
