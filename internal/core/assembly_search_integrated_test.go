package core

import (
	"testing"
)

// TestFragmentString_IntegratedHTMLDetection tests the fragment string integrated h t m l detection.
func TestFragmentString_IntegratedHTMLDetection(t *testing.T) {
	ase := &AssemblySearchEngine{
		minFragmentLength: 3,
	}

	tests := []struct {
		name        string
		input       string
		shouldFind  []string
		description string
	}{
		{
			name:        "Regular error message",
			input:       "Error: Failed to connect to database",
			shouldFind:  []string{"Error", "Failed", "connect", "database"},
			description: "Should extract words from regular text",
		},
		{
			name:        "HTML div with class",
			input:       `<div className="error-message">Error occurred</div>`,
			shouldFind:  []string{"div", "className", "error", "message", "Error", "occurred"},
			description: "Should automatically detect HTML and extract tags, attributes, and content",
		},
		{
			name:        "JSX with expression",
			input:       `<Button onClick={handleClick}>Submit</Button>`,
			shouldFind:  []string{"Button", "onClick", "handleClick", "Submit"},
			description: "Should extract React component, props, and content",
		},
		{
			name:        "API path",
			input:       "/api/v1/users/123/profile",
			shouldFind:  []string{"api", "users", "123", "profile"},
			description: "Should split paths on slashes",
		},
		{
			name:        "Complex JSX",
			input:       `<div data-testid="modal" className="error-modal">{error.message}</div>`,
			shouldFind:  []string{"div", "data-testid", "modal", "className", "error", "message", "error.message"},
			description: "Should handle data attributes and JSX expressions",
		},
		{
			name:        "Mixed content",
			input:       `Error in <span className="code">database.connect()</span>`,
			shouldFind:  []string{"Error", "span", "className", "code", "database", "connect"},
			description: "Should handle mixed HTML and text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragments := ase.fragmentString(tt.input, 3)

			t.Logf("%s", tt.description)
			t.Logf("Input: %s", tt.input)
			t.Logf("Fragments: %v", fragments)

			missing := []string{}
			for _, expected := range tt.shouldFind {
				found := false
				for _, frag := range fragments {
					if frag == expected {
						found = true
						break
					}
				}
				if !found {
					missing = append(missing, expected)
				}
			}

			if len(missing) > 0 {
				t.Errorf("Missing expected fragments: %v", missing)
			}
		})
	}
}

// TestFragmentString_JSXRealWorld tests the fragment string j s x real world.
func TestFragmentString_JSXRealWorld(t *testing.T) {
	ase := &AssemblySearchEngine{
		minFragmentLength: 3,
	}

	// Real-world JSX examples
	tests := []struct {
		name       string
		jsx        string
		rendered   string
		shouldFind []string
	}{
		{
			name: "Error modal",
			jsx: `<Modal show={showError} onHide={handleClose}>
				<Modal.Header closeButton>
					<Modal.Title>Error</Modal.Title>
				</Modal.Header>
				<Modal.Body>{errorMessage}</Modal.Body>
			</Modal>`,
			rendered:   `<div class="modal">Error: File not found</div>`,
			shouldFind: []string{"div", "modal", "Error", "File", "not", "found"},
		},
		{
			name: "User card",
			jsx: `<Card className="user-card">
				<Card.Body>
					<h5>{user.name}</h5>
					<p>{user.email}</p>
				</Card.Body>
			</Card>`,
			rendered:   `<div class="user-card"><h5>John Doe</h5><p>john@example.com</p></div>`,
			shouldFind: []string{"div", "user", "card", "h5", "John", "Doe", "john", "example", "com"},
		},
		{
			name: "Alert component",
			jsx: `<Alert variant="danger" dismissible onClose={() => setShow(false)}>
				<Alert.Heading>Oh snap!</Alert.Heading>
				<p>Something went wrong</p>
			</Alert>`,
			rendered:   `<div class="alert alert-danger">Oh snap! Something went wrong</div>`,
			shouldFind: []string{"div", "alert", "danger", "snap", "Something", "went", "wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that we can find fragments from the rendered HTML
			fragments := ase.fragmentString(tt.rendered, 3)

			t.Logf("JSX: %s", tt.jsx)
			t.Logf("Rendered HTML: %s", tt.rendered)
			t.Logf("Extracted fragments: %v", fragments)

			found := 0
			for _, expected := range tt.shouldFind {
				for _, frag := range fragments {
					if frag == expected {
						found++
						break
					}
				}
			}

			coverage := float64(found) / float64(len(tt.shouldFind)) * 100
			t.Logf("Fragment coverage: %.1f%% (%d/%d)", coverage, found, len(tt.shouldFind))

			if coverage < 80 {
				t.Errorf("Expected at least 80%% fragment coverage, got %.1f%%", coverage)
			}
		})
	}
}
