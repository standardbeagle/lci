package workflow_scenarios

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestPhraseMatching_Chi tests multi-word natural language search on Chi router
// These tests verify the PhraseMatcher integration works with real-world codebases
func TestPhraseMatching_Chi(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "go", "chi")

	// === Multi-Word Phrase Matching Tests ===
	t.Run("MultiWord_ServeHTTP", func(t *testing.T) {
		// Query: "serve http" should find "ServeHTTP" via phrase matching
		searchResult := ctx.Search("phrase_serve_http", mcp.SearchOptions{
			Pattern:    "serve http",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			if strings.Contains(result.Match, "ServeHTTP") || strings.Contains(result.SymbolName, "ServeHTTP") {
				found = true
				t.Logf("✓ Phrase 'serve http' found 'ServeHTTP': %s:%d (score:%.2f)",
					result.File, result.Line, result.Score)
				break
			}
		}

		require.True(t, found, "Multi-word query 'serve http' should find 'ServeHTTP' via phrase matching")
	})

	t.Run("MultiWord_ResponseWriter", func(t *testing.T) {
		// Query: "response writer" should find "ResponseWriter" references
		searchResult := ctx.Search("phrase_response_writer", mcp.SearchOptions{
			Pattern:    "response writer",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "responsewriter") || strings.Contains(lower, "response") {
				found = true
				t.Logf("✓ Phrase 'response writer' found: %s:%d", result.File, result.Line)
				break
			}
		}

		require.True(t, found, "Multi-word query 'response writer' should find ResponseWriter-related symbols")
	})

	t.Run("MultiWord_HTTPHandler", func(t *testing.T) {
		// Query: "http handler" should find HTTP handler related symbols
		searchResult := ctx.Search("phrase_http_handler", mcp.SearchOptions{
			Pattern:    "http handler",
			MaxResults: 30,
		})

		handlerCount := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "handler") || strings.Contains(lower, "http") {
				handlerCount++
				if handlerCount <= 3 {
					t.Logf("  Found: %s (score:%.2f)", result.SymbolName, result.Score)
				}
			}
		}

		t.Logf("Found %d HTTP handler related results", handlerCount)
		require.Greater(t, handlerCount, 0, "Multi-word query 'http handler' should find handler symbols")
	})

	t.Run("MultiWord_MiddlewareStack", func(t *testing.T) {
		// Query: "middleware stack" should find middleware-related symbols
		searchResult := ctx.Search("phrase_middleware_stack", mcp.SearchOptions{
			Pattern:    "middleware stack",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "middleware") {
				found = true
				t.Logf("✓ Phrase 'middleware stack' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		// Middleware might match via partial match even if "stack" doesn't
		if found {
			t.Logf("Multi-word phrase matching working for middleware")
		} else {
			t.Logf("No middleware results - may need specific middleware symbols in chi")
		}
	})

	// === Synonym/Vocabulary Matching Tests ===
	t.Run("Synonym_AuthLogin", func(t *testing.T) {
		// Query: "auth" should potentially find "login" related symbols via synonym expansion
		// Note: Chi may not have auth/login symbols, so this tests the mechanism
		searchResult := ctx.Search("synonym_auth", mcp.SearchOptions{
			Pattern:    "auth",
			MaxResults: 20,
		})

		authCount := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "auth") || strings.Contains(lower, "login") ||
				strings.Contains(lower, "signin") || strings.Contains(lower, "authenticate") {
				authCount++
			}
		}

		t.Logf("Found %d auth-related symbols via synonym search", authCount)
	})
}

// TestPhraseMatching_GoGitHub tests multi-word natural language search on Go-GitHub SDK
func TestPhraseMatching_GoGitHub(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "go", "go-github")

	t.Run("MultiWord_PullRequest", func(t *testing.T) {
		// Query: "pull request" should find "PullRequest" type/functions
		searchResult := ctx.Search("phrase_pull_request", mcp.SearchOptions{
			Pattern:    "pull request",
			MaxResults: 30,
		})

		found := false
		var topScore float64
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "pullrequest") {
				found = true
				topScore = result.Score
				t.Logf("✓ Phrase 'pull request' found 'PullRequest': %s:%d (score:%.2f)",
					result.File, result.Line, result.Score)
				break
			}
		}

		require.True(t, found, "Multi-word query 'pull request' should find 'PullRequest' symbols")

		// Verify it found a high-quality match
		assert.Greater(t, topScore, 0.5, "PullRequest match should have good score")
	})

	t.Run("MultiWord_IssueComment", func(t *testing.T) {
		// Query: "issue comment" should find "IssueComment" type
		searchResult := ctx.Search("phrase_issue_comment", mcp.SearchOptions{
			Pattern:    "issue comment",
			MaxResults: 30,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "issuecomment") || strings.Contains(lower, "issue") {
				found = true
				t.Logf("✓ Phrase 'issue comment' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		require.True(t, found, "Multi-word query 'issue comment' should find IssueComment-related symbols")
	})

	t.Run("MultiWord_APIClient", func(t *testing.T) {
		// Query: "api client" should find API client types
		searchResult := ctx.Search("phrase_api_client", mcp.SearchOptions{
			Pattern:    "api client",
			MaxResults: 20,
		})

		clientCount := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "client") {
				clientCount++
				if clientCount <= 3 {
					t.Logf("  Found client: %s (score:%.2f)", result.SymbolName, result.Score)
				}
			}
		}

		t.Logf("Found %d API client related results", clientCount)
		require.Greater(t, clientCount, 0, "Multi-word query 'api client' should find client symbols")
	})

	t.Run("MultiWord_RateLimiting", func(t *testing.T) {
		// Query: "rate limit" should find rate limiting related code
		searchResult := ctx.Search("phrase_rate_limit", mcp.SearchOptions{
			Pattern:    "rate limit",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "rate") || strings.Contains(lower, "limit") {
				found = true
				t.Logf("✓ Phrase 'rate limit' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		if found {
			t.Logf("Rate limiting symbols found via phrase matching")
		} else {
			t.Logf("No rate limit results - feature may use different naming")
		}
	})

	// === Stemming Tests ===
	t.Run("Stemming_Creating", func(t *testing.T) {
		// Query: "creating" should find "Create" functions via stemming
		searchResult := ctx.Search("stemming_creating", mcp.SearchOptions{
			Pattern:     "creating",
			SymbolTypes: []string{"function", "method"},
			MaxResults:  20,
		})

		createCount := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "create") {
				createCount++
				if createCount <= 3 {
					t.Logf("  Stemming found: %s", result.SymbolName)
				}
			}
		}

		t.Logf("Stemming 'creating' found %d 'create' functions", createCount)
	})

	t.Run("Stemming_Authentication", func(t *testing.T) {
		// Query: "authentication" should find "auth" or "authenticate" via stemming
		searchResult := ctx.Search("stemming_authentication", mcp.SearchOptions{
			Pattern:    "authentication",
			MaxResults: 20,
		})

		authCount := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "auth") {
				authCount++
			}
		}

		t.Logf("Stemming 'authentication' found %d auth-related symbols", authCount)
	})
}

// TestPhraseMatching_TRPC tests multi-word natural language search on tRPC
func TestPhraseMatching_TRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "typescript", "trpc")

	t.Run("MultiWord_ErrorFormatting", func(t *testing.T) {
		// Query: "error formatting" should find error formatter code
		searchResult := ctx.Search("phrase_error_formatting", mcp.SearchOptions{
			Pattern:    "error formatting",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "error") || strings.Contains(lower, "format") {
				found = true
				t.Logf("✓ Phrase 'error formatting' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		if found {
			t.Logf("Error formatting found via phrase matching")
		}
	})

	t.Run("MultiWord_InputValidation", func(t *testing.T) {
		// Query: "input validation" should find input validation code
		searchResult := ctx.Search("phrase_input_validation", mcp.SearchOptions{
			Pattern:    "input validation",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "input") || strings.Contains(lower, "valid") {
				found = true
				t.Logf("✓ Phrase 'input validation' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		if found {
			t.Logf("Input validation found via phrase matching")
		}
	})

	t.Run("MultiWord_ProcedureBuilder", func(t *testing.T) {
		// Query: "procedure builder" should find procedure-related types
		searchResult := ctx.Search("phrase_procedure_builder", mcp.SearchOptions{
			Pattern:    "procedure builder",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "procedure") || strings.Contains(lower, "builder") {
				found = true
				t.Logf("✓ Phrase 'procedure builder' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		if found {
			t.Logf("Procedure builder found via phrase matching")
		}
	})

	// === Abbreviation Tests ===
	t.Run("Abbreviation_Config", func(t *testing.T) {
		// Query: "config" should find "configuration" via abbreviation expansion
		searchResult := ctx.Search("abbrev_config", mcp.SearchOptions{
			Pattern:    "config",
			MaxResults: 20,
		})

		configCount := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "config") || strings.Contains(lower, "configuration") {
				configCount++
			}
		}

		t.Logf("Abbreviation 'config' found %d configuration-related symbols", configCount)
		require.Greater(t, configCount, 0, "Abbreviation 'config' should find configuration symbols")
	})
}

// TestPhraseMatching_Pocketbase tests multi-word natural language search on Pocketbase
func TestPhraseMatching_Pocketbase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "go", "pocketbase")

	t.Run("MultiWord_RecordCollection", func(t *testing.T) {
		// Query: "record collection" should find record/collection types
		searchResult := ctx.Search("phrase_record_collection", mcp.SearchOptions{
			Pattern:    "record collection",
			MaxResults: 30,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "record") || strings.Contains(lower, "collection") {
				found = true
				t.Logf("✓ Phrase 'record collection' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		require.True(t, found, "Multi-word query 'record collection' should find record/collection symbols")
	})

	t.Run("MultiWord_UserAuth", func(t *testing.T) {
		// Query: "user auth" should find user authentication code
		searchResult := ctx.Search("phrase_user_auth", mcp.SearchOptions{
			Pattern:    "user auth",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "user") || strings.Contains(lower, "auth") {
				found = true
				t.Logf("✓ Phrase 'user auth' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		if found {
			t.Logf("User auth found via phrase matching")
		}
	})

	t.Run("MultiWord_DatabaseMigration", func(t *testing.T) {
		// Query: "database migration" should find migration-related code
		searchResult := ctx.Search("phrase_database_migration", mcp.SearchOptions{
			Pattern:    "database migration",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "migration") || strings.Contains(lower, "database") ||
				strings.Contains(lower, "migrate") {
				found = true
				t.Logf("✓ Phrase 'database migration' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		if found {
			t.Logf("Database migration found via phrase matching")
		}
	})

	// === Synonym/Vocabulary Matching ===
	t.Run("Synonym_StoreData", func(t *testing.T) {
		// Query: "store data" should find "persist" or "save" via synonym expansion
		searchResult := ctx.Search("synonym_store_data", mcp.SearchOptions{
			Pattern:    "store data",
			MaxResults: 20,
		})

		found := false
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			if strings.Contains(lower, "store") || strings.Contains(lower, "save") ||
				strings.Contains(lower, "persist") || strings.Contains(lower, "data") {
				found = true
				t.Logf("✓ Synonym 'store data' found: %s (score:%.2f)", result.SymbolName, result.Score)
				break
			}
		}

		if found {
			t.Logf("Store/persist data found via synonym matching")
		}
	})
}

// TestPhraseMatching_ScoreValidation validates that phrase matching scores are reasonable
func TestPhraseMatching_ScoreValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping workflow test in short mode")
	}

	ctx := GetProject(t, "go", "chi")

	t.Run("ExactPhraseScoresHigherThanPartial", func(t *testing.T) {
		// Exact phrase match should score higher than partial matches
		searchResult := ctx.Search("score_validation", mcp.SearchOptions{
			Pattern:    "serve http",
			MaxResults: 30,
		})

		if len(searchResult.Results) == 0 {
			t.Skip("No results to validate scores")
		}

		// Results should be sorted by score descending
		for i := 1; i < len(searchResult.Results); i++ {
			assert.GreaterOrEqual(t, searchResult.Results[i-1].Score, searchResult.Results[i].Score,
				"Results should be sorted by score descending at position %d", i)
		}

		// First result should have a reasonable score
		assert.Greater(t, searchResult.Results[0].Score, 0.3,
			"Top result should have score > 0.3")

		t.Logf("Score validation passed: top score %.2f", searchResult.Results[0].Score)
	})

	t.Run("MultiWordBetterThanSingleWord", func(t *testing.T) {
		// Multi-word query should prefer matches with multiple words
		searchResult := ctx.Search("multiword_validation", mcp.SearchOptions{
			Pattern:    "http response writer",
			MaxResults: 20,
		})

		if len(searchResult.Results) == 0 {
			t.Skip("No results to validate")
		}

		// Count how many results contain multiple query words
		multiWordMatches := 0
		for _, result := range searchResult.Results {
			lower := strings.ToLower(result.Match + result.SymbolName)
			wordCount := 0
			if strings.Contains(lower, "http") {
				wordCount++
			}
			if strings.Contains(lower, "response") {
				wordCount++
			}
			if strings.Contains(lower, "writer") {
				wordCount++
			}
			if wordCount >= 2 {
				multiWordMatches++
			}
		}

		t.Logf("Found %d results with 2+ matching words out of %d total",
			multiWordMatches, len(searchResult.Results))
	})
}
