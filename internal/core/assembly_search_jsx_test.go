package core

import (
	"testing"
)

// TestAssemblySearch_JSXContent tests the assembly search j s x content.
func TestAssemblySearch_JSXContent(t *testing.T) {
	ase := &AssemblySearchEngine{
		minFragmentLength: 3,
	}

	tests := []struct {
		name     string
		html     string
		expected []string
		desc     string
	}{
		{
			name: "Simple JSX div with class",
			html: `<div className="error-message">Error: User not found</div>`,
			expected: []string{
				"div", "className", "error-message",
				"Error", "User", "not", "found",
			},
			desc: "Should extract tag names, attributes, and content",
		},
		{
			name: "Nested JSX with dynamic content",
			html: `<div className="user-card"><h1>{user.name}</h1><p>{user.bio}</p></div>`,
			expected: []string{
				"div", "className", "user-card",
				"h1", "user.name", "user", "name",
				"p", "user.bio", "bio",
			},
			desc: "Should handle nested tags and JSX expressions",
		},
		{
			name: "React component with props",
			html: `<Button variant="primary" onClick={handleClick}>Submit</Button>`,
			expected: []string{
				"Button", "variant", "primary",
				"onClick", "handleClick", "Submit",
			},
			desc: "Should extract component names and props",
		},
		{
			name: "HTML with data attributes",
			html: `<div data-testid="error-modal" aria-label="Error message">Something went wrong</div>`,
			expected: []string{
				"div", "data-testid", "error-modal",
				"aria-label", "Error", "message",
				"Something", "went", "wrong",
			},
			desc: "Should handle data attributes and aria labels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with current implementation
			fragments := ase.fragmentString(tt.html, 3)

			t.Logf("Current fragments for %s: %v", tt.name, fragments)

			// Check what we're missing
			missing := []string{}
			for _, expected := range tt.expected {
				found := false
				for _, frag := range fragments {
					if frag == expected {
						found = true
						break
					}
				}
				if !found && len(expected) >= 3 {
					missing = append(missing, expected)
				}
			}

			if len(missing) > 0 {
				t.Logf("Missing important fragments: %v", missing)
			}
		})
	}
}

// TestAssemblySearch_JSXPatterns tests the assembly search j s x patterns.
func TestAssemblySearch_JSXPatterns(t *testing.T) {
	tests := []struct {
		name     string
		jsx      string
		rendered string
		desc     string
	}{
		{
			name:     "Conditional rendering",
			jsx:      `{isError && <div className="error">Failed to load</div>}`,
			rendered: `<div className="error">Failed to load</div>`,
			desc:     "Conditional JSX rendering",
		},
		{
			name:     "Map rendering",
			jsx:      `{items.map(item => <li key={item.id}>{item.name}</li>)}`,
			rendered: `<li>Item One</li><li>Item Two</li>`,
			desc:     "Dynamic list rendering",
		},
		{
			name:     "Template literal in className",
			jsx:      "className={`status-${status}`}",
			rendered: `className="status-active"`,
			desc:     "Dynamic class names",
		},
	}

	ase := &AssemblySearchEngine{
		minFragmentLength: 3,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragments := ase.fragmentString(tt.rendered, 3)
			t.Logf("%s\nJSX: %s\nRendered: %s\nFragments: %v\n",
				tt.desc, tt.jsx, tt.rendered, fragments)
		})
	}
}
