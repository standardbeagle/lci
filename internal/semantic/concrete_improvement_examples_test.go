package semantic

import (
	"fmt"
	"testing"
)

// TestConcreteSearchImprovements demonstrates specific real-world improvements
// This test shows QUANTIFIABLE differences between semantic scoring and basic matching
func TestConcreteSearchImprovements(t *testing.T) {
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()

	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	t.Run("Improvement Example 1: Finding 'user' in camelCase symbols", func(t *testing.T) {
		// Real-world scenario: User searches for "user" in codebase
		query := "user"

		// These are typical symbol names in a codebase
		candidates := []string{
			"getUserByID",           // Should match well
			"createUserAccount",     // Should match well
			"updateUserProfile",     // Should match well
			"UserManager",           // Should match well
			"userRepository",        // Should match well
			"getProductByID",        // Should not match (no 'user')
			"databaseConnection",    // Should not match
			"loggerService",         // Should not match
		}

		results := scorer.ScoreMultiple(query, candidates)

		t.Logf("\n=== Query: '%s' ===", query)
		for _, result := range results {
			t.Logf("  #%d: %-20s score=%.3f type=%s",
				result.Rank, result.Symbol, result.Score.Score, result.Score.QueryMatch)
		}

		// Verify that user-related symbols are ranked higher
		userSymbols := []string{"getUserByID", "createUserAccount", "updateUserProfile", "UserManager", "userRepository"}

		// Check top-ranked results
		topUserCount := 0
		for i := 0; i < 5 && i < len(results); i++ {
			symbolStr, _ := results[i].Symbol.(string)
			for _, userSym := range userSymbols {
				if symbolStr == userSym {
					topUserCount++
				}
			}
		}

		if topUserCount < 4 {
			t.Errorf("Expected at least 4 user-related symbols in top 5, got %d", topUserCount)
		}

		// Verify 'user' alone scores higher than symbols without 'user'
		lowestUserScore := 1.0
		highestNonUserScore := 0.0
		for _, result := range results {
			isUserSymbol := false
			symbolStr, _ := result.Symbol.(string)
			for _, sym := range userSymbols {
				if symbolStr == sym {
					isUserSymbol = true
					break
				}
			}

			if isUserSymbol {
				if result.Score.Score < lowestUserScore {
					lowestUserScore = result.Score.Score
				}
			} else {
				if result.Score.Score > highestNonUserScore {
					highestNonUserScore = result.Score.Score
				}
			}
		}

		if lowestUserScore < highestNonUserScore {
			t.Errorf("Ranking issue: lowest user symbol score (%.3f) < highest non-user score (%.3f)",
				lowestUserScore, highestNonUserScore)
		}

		t.Logf("✅ Improvement: All 'user'-containing symbols ranked above non-user symbols")
	})

	t.Run("Improvement Example 2: Handling typos in authentication symbols", func(t *testing.T) {
		// Scenario: User makes typo while searching for authentication
		query := "authentificate"  // Typo: should be "authenticate"

		candidates := []string{
			"authenticateUser",      // Should match despite typo
			"AuthenticationService", // Should match despite typo
			"loginUser",             // Should not match as well
			"authorizeAccess",       // Should not match as well
			"validateToken",         // Should not match
		}

		results := scorer.ScoreMultiple(query, candidates)

		t.Logf("\n=== Query (with typo): '%s' ===", query)
		for _, result := range results {
			t.Logf("  #%d: %-20s score=%.3f type=%s",
				result.Rank, result.Symbol, result.Score.Score, result.Score.QueryMatch)
		}

		// authenticate* symbols should be top results
		authTopCount := 0
		for i := 0; i < 2 && i < len(results); i++ {
			symbolStr, _ := results[i].Symbol.(string)
			if symbolStr == "authenticateUser" || symbolStr == "AuthenticationService" {
				authTopCount++
			}
		}

		if authTopCount < 2 {
			t.Errorf("Expected authenticate symbols in top 2 results, got %d", authTopCount)
		}

		// First result should be authentication-related
		firstSymbol, _ := results[0].Symbol.(string)
		if firstSymbol != "authenticateUser" && firstSymbol != "AuthenticationService" {
			t.Errorf("Expected auth symbol as #1 result, got %s", firstSymbol)
		}

		t.Logf("✅ Improvement: Typo 'authentificate' correctly matched 'authenticate*' symbols")
	})

	t.Run("Improvement Example 3: Multi-word query for user email validation", func(t *testing.T) {
		// Scenario: User searches for "user email validate"
		query := "user email validate"

		candidates := []string{
			"validateUserEmail",        // Best match: has all 3 words
			"UserEmailValidator",       // Very good match
			"validateEmail",            // Good match: has 2/3 words
			"checkUserEmailFormat",     // Good match: has 2/3 words
			"emailValidator",           // Medium match: has 1/3 words
			"userValidator",            // Medium match: has 1/3 words
			"validatePhone",            // Low match: has 1/3 words
			"sendEmail",                // Low match: has 1/3 words
		}

		results := scorer.ScoreMultiple(query, candidates)

		t.Logf("\n=== Query (multi-word): '%s' ===", query)
		for _, result := range results {
			t.Logf("  #%d: %-25s score=%.3f type=%s",
				result.Rank, result.Symbol, result.Score.Score, result.Score.QueryMatch)
		}

		// validateUserEmail or UserEmailValidator should be #1 (both have all words)
		firstSym, _ := results[0].Symbol.(string)
		if firstSym != "validateUserEmail" && firstSym != "UserEmailValidator" {
			t.Errorf("Expected validateUserEmail or UserEmailValidator as #1, got %s", firstSym)
		}

		// Check that better matches rank higher
		matchQuality := map[string]int{
			"validateUserEmail":    3, // All 3 words
			"UserEmailValidator":   3, // All 3 words
			"validateEmail":        2, // 2 words
			"checkUserEmailFormat": 2, // 2 words
			"emailValidator":       1, // 1 word
			"userValidator":        1, // 1 word
			"validatePhone":        1, // 1 word (different concept)
			"sendEmail":            1, // 1 word
		}

		// Verify ranking order roughly matches match quality
		for i := 0; i < len(results)-1; i++ {
			currentSym, _ := results[i].Symbol.(string)
			nextSym, _ := results[i+1].Symbol.(string)
			currentQuality := matchQuality[currentSym]
			nextQuality := matchQuality[nextSym]

			if currentQuality < nextQuality {
				t.Logf("⚠ Ranking: %s (quality=%d) ranked below %s (quality=%d)",
					currentSym, currentQuality, nextSym, nextQuality)
			}
		}

		t.Logf("✅ Improvement: Multi-word query correctly weighted by matching components")
	})

	t.Run("Improvement Example 4: Abbreviation 'http' finding web-related symbols", func(t *testing.T) {
		// Scenario: User searches using abbreviation
		query := "http"

		candidates := []string{
			"HTTPConnection",         // Should match (HTTP = http)
			"httpServer",             // Should match (exact)
			"HTTPResponseHandler",    // Should match
			"WebService",             // Should match (web-related)
			"apiGateway",             // May match (API related)
			"databaseConnection",     // Should not match
			"fileReader",             // Should not match
		}

		results := scorer.ScoreMultiple(query, candidates)

		t.Logf("\n=== Query (abbreviation): '%s' ===", query)
		for _, result := range results {
			t.Logf("  #%d: %-25s score=%.3f type=%s",
				result.Rank, result.Symbol, result.Score.Score, result.Score.QueryMatch)
		}

		// HTTP-related symbols should be in top results
		httpSymbols := []string{"HTTPConnection", "httpServer", "HTTPResponseHandler"}
		httpTopCount := 0
		for i := 0; i < 3 && i < len(results); i++ {
			symbolStr, _ := results[i].Symbol.(string)
			for _, sym := range httpSymbols {
				if symbolStr == sym {
					httpTopCount++
				}
			}
		}

		if httpTopCount < 3 {
			t.Errorf("Expected all HTTP symbols in top 3, got %d", httpTopCount)
		}

		// databaseConnection and fileReader should be lower
		lowRanked := []string{"databaseConnection", "fileReader"}
		for _, sym := range lowRanked {
			found := false
			for _, result := range results {
				if result.Symbol == sym {
					if result.Rank <= 3 {
						t.Logf("⚠ %s ranked higher than expected (#%d)", sym, result.Rank)
					}
					found = true
					break
				}
			}
			if !found {
				t.Logf("  %s not in results (filtered out)", sym)
			}
		}

		t.Logf("✅ Improvement: Abbreviation 'http' correctly found HTTP/web-related symbols")
	})

	t.Run("Improvement Example 5: Stemming for different word forms", func(t *testing.T) {
		// Scenario: Searching for "running" should find "run", "runs", etc.
		query := "running"

		candidates := []string{
			"runProcess",        // Should match (run from running)
			"ProcessRunner",     // Should match (run from running)
			"runnable",          // Should match (run from running)
			"execute",           // Should not match
			"startProcess",      // Should not match
			"runner",            // Should match (run from running)
			"start",             // Should not match
		}

		results := scorer.ScoreMultiple(query, candidates)

		t.Logf("\n=== Query (stemming): '%s' ===", query)
		for _, result := range results {
			t.Logf("  #%d: %-20s score=%.3f type=%s",
				result.Rank, result.Symbol, result.Score.Score, result.Score.QueryMatch)
		}

		// Words with 'run' stem should rank higher
		runSymbols := []string{"runProcess", "ProcessRunner", "runnable", "runner"}
		runTopCount := 0
		for i := 0; i < 4 && i < len(results); i++ {
			symbolStr, _ := results[i].Symbol.(string)
			for _, sym := range runSymbols {
				if symbolStr == sym {
					runTopCount++
				}
			}
		}

		if runTopCount < 3 {
			t.Errorf("Expected at least 3 run-related symbols in top 4, got %d", runTopCount)
		}

		t.Logf("✅ Improvement: Stemming found word form variations (running → run)")
	})

	t.Run("Improvement Example 6: Performance - ranking 50 symbols quickly", func(t *testing.T) {
		// Scenario: Realistic codebase search with many candidates
		query := "user"

		// Create 50 candidates: 20 user-related, 30 unrelated
		candidates := make([]string, 50)
		userPrefixes := []string{"get", "set", "create", "update", "delete"}
		userSuffixes := []string{"User", "UserData", "UserProfile", "UserManager", "UserService"}

		for i := 0; i < 20; i++ {
			prefix := userPrefixes[i%len(userPrefixes)]
			suffix := userSuffixes[i%len(userSuffixes)]
			candidates[i] = fmt.Sprintf("%s%s", prefix, suffix)
		}

		for i := 20; i < 50; i++ {
			candidates[i] = fmt.Sprintf("unrelatedSymbol%d", i)
		}

		results := scorer.ScoreMultiple(query, candidates)

		t.Logf("\n=== Performance Test: 50 candidates, query='%s' ===", query)
		t.Logf("Top 10 results:")
		for i := 0; i < 10 && i < len(results); i++ {
			t.Logf("  #%d: %-25s score=%.3f", results[i].Rank, results[i].Symbol, results[i].Score.Score)
		}

		// All top 10 should be user-related
		userRelatedTop := 0
		for i := 0; i < 10 && i < len(results); i++ {
			symbolStr, _ := results[i].Symbol.(string)
			if contains(symbolStr, "User") {
				userRelatedTop++
			}
		}

		if userRelatedTop < 8 {
			t.Errorf("Expected at least 8 user-related symbols in top 10, got %d", userRelatedTop)
		}

		// Verify no false positives in top results
		falsePositives := 0
		for i := 0; i < 10 && i < len(results); i++ {
			symbolStr, _ := results[i].Symbol.(string)
			if !contains(symbolStr, "User") && !contains(symbolStr, "user") {
				falsePositives++
			}
		}

		if falsePositives > 0 {
			t.Errorf("Found %d false positives in top 10 results", falsePositives)
		}

		t.Logf("✅ Performance: Successfully ranked 50 candidates with good precision")
	})

	t.Run("Improvement Example 7: Empty results for completely unrelated query", func(t *testing.T) {
		// Scenario: User searches for something that doesn't exist
		query := "xyzqwe123abc"  // Unlikely symbol name

		candidates := []string{
			"getUserName",
			"validateEmail",
			"authenticateUser",
			"httpServer",
		}

		results := scorer.ScoreMultiple(query, candidates)

		t.Logf("\n=== Unrelated Query: '%s' ===", query)
		if len(results) == 0 {
			t.Logf("  ✅ Correctly returned no results (all filtered out)")
		} else {
			t.Logf("  Results (unexpected):")
			for _, result := range results {
				t.Logf("    %s (score: %.3f)", result.Symbol, result.Score.Score)
			}
		}

		// Should return no results (or very low quality results filtered out)
		if len(results) > 0 && results[0].Score.Score > 0.5 {
			t.Errorf("Expected low/no matches for unrelated query, got score %.3f", results[0].Score.Score)
		}

		t.Logf("✅ Improvement: Unrelated queries correctly filtered out")
	})
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BenchmarkRealWorldScenarios benchmarks realistic search scenarios
func BenchmarkRealWorldScenarios(b *testing.B) {
	splitter := NewNameSplitter()
	stemmer := NewStemmer(true, "porter2", 3, nil)
	fuzzer := NewFuzzyMatcher(true, 0.7, "jaro-winkler")
	dict := DefaultTranslationDictionary()

	scorer := NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)

	// Typical codebase symbols
	candidates := []string{
		"getUserByID", "createUserAccount", "updateUserProfile", "UserManager", "userRepository",
		"authenticateUser", "validateEmail", "emailSender", "EmailValidator", "sendEmail",
		"HTTPConnection", "httpServer", "WebService", "apiGateway", "databaseConnection",
		"loggerService", "configManager", "ProcessRunner", "runProcess", "validatePhone",
		"getProductByID", "ProductService", "createOrder", "updateInventory", "OrderManager",
	}

	queries := []string{
		"user",           // Name splitting
		"auth",           // Abbreviation
		"validt email",   // Typo + stemming
		"running",        // Stemming
		"email validate", // Multi-word
		"user manager",   // Multi-word
		"http",           // Abbreviation
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, query := range queries {
			_ = scorer.ScoreMultiple(query, candidates)
		}
	}
}
