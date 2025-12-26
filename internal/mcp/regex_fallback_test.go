package mcp

import (
	"strings"
	"testing"
)

func TestLooksLikeRegex(t *testing.T) {
	tests := []struct {
		pattern  string
		expected bool
	}{
		// === Pipe (OR) patterns ===
		{"foo|bar", true},
		{"NewRouter|NewMux", true},
		{"auth|login|session", true},
		{"|", true},
		{"a|", true},
		{"|b", true},

		// === Character classes ===
		{"[A-Z]Handler", true},
		{"log[0-9]+", true},
		{"[abc]", true},
		{"[^abc]", true},
		{"test[_-]file", true},

		// === Anchors ===
		{"^func", true},
		{"Handler$", true},
		{"^main$", true},

		// === Backslash escapes ===
		{`\d+`, true},
		{`\w+`, true},
		{`\s*`, true},
		{`\bword\b`, true},
		{`\D`, true},
		{`\W`, true},
		{`\S`, true},
		{`\B`, true},
		{`errors\.New`, true},   // escaped dot = regex
		{`fmt\.Errorf`, true},   // escaped dot = regex
		{`foo\.bar\.baz`, true}, // multiple escaped dots
		{`\*`, true},            // escaped asterisk
		{`\+`, true},            // escaped plus
		{`\?`, true},            // escaped question
		{`\(`, true},            // escaped paren
		{`\[`, true},            // escaped bracket
		{`\{`, true},            // escaped brace
		{`\^`, true},            // escaped caret
		{`\$`, true},            // escaped dollar
		{`\|`, true},            // escaped pipe
		{`\\`, true},            // escaped backslash

		// === Dot quantifiers ===
		{".+", true},
		{".*", true},
		{"foo.+bar", true},
		{"test.*end", true},

		// === Parentheses (grouping) - only with regex-specific content ===
		{"(foo|bar)", true}, // pipe inside parens = regex
		{"(?:test)", true},  // non-capturing group = regex
		{"(?=lookahead)", true},
		{"(?!negation)", true},
		{"(a|b|c)", true}, // alternation inside parens

		// === Curly brace quantifiers ===
		{"a{2}", true},
		{"a{2,5}", true},
		{"a{2,}", true},
		{"test{3}pattern", true},

		// === Patterns that DON'T look like regex ===
		{"foo", false},
		{"NewRouter", false},
		{"foo*bar", false}, // asterisk could be glob, not regex
		{"foo.bar", false}, // likely qualified name
		{"foo_bar", false},
		{"foo-bar", false},
		{"", false},
		{"package.Class", false},
		{"my.module.func", false},
		{"${var}", false}, // template literal, not quantifier
		{"config.json", false},
		{"file.txt", false},
		{`foo\nbar`, false}, // \n is not a regex escape we check for
		{"hello world", false},
		{"CamelCaseFunc", false},
		{"snake_case_var", false},
		{"func(", false},        // incomplete paren, likely function call search
		{"main()", false},       // function call, not regex grouping
		{"fmt.Println(", false}, // Go function call
		{"console.log(", false}, // JS function call
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := looksLikeRegex(tt.pattern)
			if result != tt.expected {
				t.Errorf("looksLikeRegex(%q) = %v, want %v", tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestRegexFallbackScoreMultiplier(t *testing.T) {
	// Verify the multiplier is set reasonably (between 0 and 1)
	if RegexFallbackScoreMultiplier <= 0 || RegexFallbackScoreMultiplier >= 1 {
		t.Errorf("RegexFallbackScoreMultiplier should be between 0 and 1, got %v", RegexFallbackScoreMultiplier)
	}

	// Verify it reduces scores significantly
	if RegexFallbackScoreMultiplier > 0.75 {
		t.Errorf("RegexFallbackScoreMultiplier should be <= 0.75 to clearly differentiate fallback results, got %v", RegexFallbackScoreMultiplier)
	}
}

func TestValidateAndNormalizeFlags(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantNormalized   string
		wantWarningCount int
		wantWarningText  string // substring to check in warnings
	}{
		// Valid flags pass through unchanged
		{"valid single", "rx", "rx", 0, ""},
		{"valid multiple", "ci,rx,nt", "ci,rx,nt", 0, ""},
		{"valid with spaces", "ci, rx, nt", "ci,rx,nt", 0, ""},

		// Auto-correction of common mistakes
		{"regex to rx", "regex", "rx", 1, "auto-corrected to 'rx'"},
		{"regexp to rx", "regexp", "rx", 1, "auto-corrected to 'rx'"},
		{"i to ci", "i", "ci", 1, "auto-corrected to 'ci'"},
		{"case-insensitive to ci", "case-insensitive", "ci", 1, "auto-corrected to 'ci'"},
		{"invert to iv", "invert", "iv", 1, "auto-corrected to 'iv'"},
		{"word to wb", "word", "wb", 1, "auto-corrected to 'wb'"},
		{"no-tests to nt", "no-tests", "nt", 1, "auto-corrected to 'nt'"},
		{"no-comments to nc", "no-comments", "nc", 1, "auto-corrected to 'nc'"},

		// Mixed valid and invalid
		{"mix valid and alias", "ci,regex", "ci,rx", 1, "auto-corrected"},
		{"mix with unknown", "ci,unknown,rx", "ci,rx", 1, "unknown flag 'unknown' ignored"},

		// Unknown flags
		{"unknown flag", "foobar", "", 1, "unknown flag 'foobar' ignored"},
		{"multiple unknown", "foo,bar", "", 2, "unknown flag"},

		// Deduplication
		{"duplicate flag", "rx,rx", "rx", 0, ""},
		{"alias then valid", "regex,rx", "rx", 1, "auto-corrected"}, // regex corrects to rx, then rx is duplicate

		// Empty cases
		{"empty string", "", "", 0, ""},
		{"only commas", ",,,", "", 0, ""},
		{"spaces only", "  ,  ,  ", "", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, warnings := validateAndNormalizeFlags(tt.input)

			if normalized != tt.wantNormalized {
				t.Errorf("validateAndNormalizeFlags(%q) normalized = %q, want %q", tt.input, normalized, tt.wantNormalized)
			}

			if len(warnings) != tt.wantWarningCount {
				t.Errorf("validateAndNormalizeFlags(%q) warning count = %d, want %d. Warnings: %v", tt.input, len(warnings), tt.wantWarningCount, warnings)
			}

			if tt.wantWarningText != "" {
				found := false
				for _, w := range warnings {
					if strings.Contains(w, tt.wantWarningText) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateAndNormalizeFlags(%q) warnings %v don't contain %q", tt.input, warnings, tt.wantWarningText)
				}
			}
		})
	}
}

func TestFlagAliasesCompleteness(t *testing.T) {
	// Verify all aliases map to valid flags
	for alias, target := range flagAliases {
		if _, ok := validFlags[target]; !ok {
			t.Errorf("flagAliases[%q] = %q, but %q is not a valid flag", alias, target, target)
		}
	}
}

func TestValidFlagsDocumentation(t *testing.T) {
	// Verify all valid flags have non-empty descriptions
	for flag, desc := range validFlags {
		if desc == "" {
			t.Errorf("validFlags[%q] has empty description", flag)
		}
		// Verify flag is 2 characters (our convention)
		if len(flag) != 2 {
			t.Errorf("validFlags[%q] should be 2 characters, got %d", flag, len(flag))
		}
	}
}
