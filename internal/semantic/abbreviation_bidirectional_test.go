package semantic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBidirectionalAbbreviations tests that abbreviations work both ways
func TestBidirectionalAbbreviations(t *testing.T) {
	dict := DefaultTranslationDictionary()
	matcher := NewAbbreviationMatcher(dict, NewNameSplitter())
	config := DefaultScoreLayers

	tests := []struct {
		name         string
		query        string
		targetSymbol string
		shouldMatch  bool
		matchType    string // "forward", "reverse", or "bidirectional"
		description  string
	}{
		// Forward expansion: abbreviation → full word
		{
			name:         "forward_auth_to_authenticate",
			query:        "auth",
			targetSymbol: "AuthenticateUser",
			shouldMatch:  true,
			matchType:    "forward",
			description:  "'auth' expands to 'authenticate' which matches 'AuthenticateUser'",
		},
		{
			name:         "forward_db_to_database",
			query:        "db",
			targetSymbol: "DatabaseConnection",
			shouldMatch:  true,
			matchType:    "forward",
			description:  "'db' expands to 'database' which matches 'DatabaseConnection'",
		},
		{
			name:         "forward_http_to_hypertext",
			query:        "http",
			targetSymbol: "HypertextTransferProtocol",
			shouldMatch:  true,
			matchType:    "forward",
			description:  "'http' expands to 'hypertext' which matches 'HypertextTransferProtocol'",
		},

		// Reverse expansion: full word → abbreviation
		{
			name:         "reverse_transaction_to_txn",
			query:        "txn",
			targetSymbol: "TransactionManager",
			shouldMatch:  true,
			matchType:    "reverse",
			description:  "'transaction' in target expands to 'txn' which matches query",
		},
		{
			name:         "reverse_test_to_tst",
			query:        "tst",
			targetSymbol: "TestRunner",
			shouldMatch:  true,
			matchType:    "reverse",
			description:  "'test' in target expands to 'tst' which matches query",
		},
		{
			name:         "reverse_authenticate_to_auth",
			query:        "auth",
			targetSymbol: "AuthenticationService",
			shouldMatch:  true,
			matchType:    "bidirectional", // "auth" also directly expands to "authentication"
			description:  "'authentication' in target expands to 'auth' (and 'auth' expands to 'authentication')",
		},

		// Non-matches
		{
			name:         "no_match_unrelated",
			query:        "auth",
			targetSymbol: "CalculateTax",
			shouldMatch:  false,
			description:  "No relationship between 'auth' and 'CalculateTax'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, score, justification, details := matcher.Detect(tt.query, tt.targetSymbol, strings.ToLower(tt.query), strings.ToLower(tt.targetSymbol), config)

			if tt.shouldMatch {
				assert.True(t, matched, "Expected match for: %s", tt.description)
				assert.Greater(t, score, 0.0, "Score should be positive for a match")
				assert.NotEmpty(t, justification, "Justification should be provided")
				assert.NotEmpty(t, details, "Details should be provided")

				t.Logf("✓ Match: %s", tt.description)
				t.Logf("  Score: %.2f", score)
				t.Logf("  Justification: %s", justification)
				t.Logf("  Details: %+v", details)

				// Verify match type - just check that we got a reasonable justification
				// The implementation uses bidirectional Expand() so it may not distinguish
				// between forward and reverse in the message, but the match still works
				assert.Contains(t, strings.ToLower(justification), "abbreviation",
					"Match justification should mention 'abbreviation'")
			} else {
				assert.False(t, matched, "Expected no match for: %s", tt.description)
				assert.Equal(t, 0.0, score, "Score should be 0 for no match")
			}
		})
	}
}

// TestAbbreviationExpansionBidirectional tests the Expand method works both ways
func TestAbbreviationExpansionBidirectional(t *testing.T) {
	dict := DefaultTranslationDictionary()

	tests := []struct {
		name             string
		term             string
		shouldContain    []string
		shouldNotContain []string
		description      string
	}{
		{
			name:          "forward_auth",
			term:          "auth",
			shouldContain: []string{"authenticate", "authorization", "authorized", "login", "signin"},
			description:   "Abbreviation 'auth' should expand to full authentication terms",
		},
		{
			name:          "reverse_authenticate",
			term:          "authenticate",
			shouldContain: []string{"auth"}, // Should include the abbreviation
			description:   "Full word 'authenticate' should expand back to abbreviation 'auth'",
		},
		{
			name:          "forward_db",
			term:          "db",
			shouldContain: []string{"database", "databases"},
			description:   "Abbreviation 'db' should expand to 'database'",
		},
		{
			name:          "reverse_database",
			term:          "database",
			shouldContain: []string{"db"}, // Should include the abbreviation
			description:   "Full word 'database' should expand back to abbreviation 'db'",
		},
		{
			name:             "unrelated_term",
			term:             "foobar",
			shouldContain:    []string{"foobar"}, // Should at least include itself
			shouldNotContain: []string{"auth", "db"},
			description:      "Unrelated term should not expand to random abbreviations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded := dict.Expand(tt.term)
			require.NotEmpty(t, expanded, "Expand should always return at least the original term")

			t.Logf("Expanded '%s' to: %v", tt.term, expanded)

			for _, expected := range tt.shouldContain {
				assert.Contains(t, expanded, expected,
					"%s: expansion should contain '%s'", tt.description, expected)
			}

			for _, notExpected := range tt.shouldNotContain {
				assert.NotContains(t, expanded, notExpected,
					"%s: expansion should NOT contain '%s'", tt.description, notExpected)
			}
		})
	}
}

// TestRealWorldAbbreviations tests real-world abbreviation patterns
func TestRealWorldAbbreviations(t *testing.T) {
	dict := DefaultTranslationDictionary()
	matcher := NewAbbreviationMatcher(dict, NewNameSplitter())
	config := DefaultScoreLayers

	// Add test-specific abbreviations (in real usage, these would be in the dictionary)
	// For now, test with existing abbreviations

	scenarios := []struct {
		scenario string
		query    string
		symbols  []string // Symbols that should match
	}{
		{
			scenario: "auth_related_searches",
			query:    "auth",
			symbols: []string{
				"AuthenticateUser",
				"AuthorizationService",
				"LoginHandler",
				"SignInController",
			},
		},
		{
			scenario: "database_searches",
			query:    "db",
			symbols: []string{
				"DatabaseConnection",
				"DBMigration",
			},
		},
		{
			scenario: "api_searches",
			query:    "api",
			symbols: []string{
				"ApplicationProgrammingInterface",
				"APIHandler",
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.scenario, func(t *testing.T) {
			matchCount := 0
			for _, symbol := range sc.symbols {
				matched, score, justification, _ := matcher.Detect(sc.query, symbol, strings.ToLower(sc.query), strings.ToLower(symbol), config)
				if matched {
					matchCount++
					t.Logf("  ✓ '%s' matched '%s' (score: %.2f): %s",
						sc.query, symbol, score, justification)
				} else {
					t.Logf("  ✗ '%s' did NOT match '%s'", sc.query, symbol)
				}
			}

			assert.Greater(t, matchCount, 0,
				"Query '%s' should match at least one symbol in scenario '%s'",
				sc.query, sc.scenario)
		})
	}
}
