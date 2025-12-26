package semantic

import (
	"testing"
)

func TestNewStemmer(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, map[string]bool{"api": true})

	if !stemmer.IsEnabled() {
		t.Error("Stemmer should be enabled")
	}

	if stemmer.GetAlgorithm() != "porter2" {
		t.Errorf("Expected algorithm porter2, got %s", stemmer.GetAlgorithm())
	}

	if stemmer.GetMinLength() != 3 {
		t.Errorf("Expected min length 3, got %d", stemmer.GetMinLength())
	}

	if !stemmer.IsExcluded("api") {
		t.Error("api should be in exclusions")
	}
}

func TestStemDisabled(t *testing.T) {
	stemmer := NewStemmer(false, "porter2", 3, nil)

	// When disabled, should return original word
	if stemmer.Stem("running") != "running" {
		t.Error("Stemming should return original when disabled")
	}

	if stemmer.Stem("authentication") != "authentication" {
		t.Error("Stemming should return original when disabled")
	}
}

func TestStemExcluded(t *testing.T) {
	exclusions := map[string]bool{
		"api": true,
		"db":  true,
		"uri": true,
	}

	stemmer := NewStemmer(true, "porter2", 3, exclusions)

	// Excluded words should not be stemmed
	if stemmer.Stem("api") != "api" {
		t.Error("Excluded word 'api' should not be stemmed")
	}

	if stemmer.Stem("db") != "db" {
		t.Error("Excluded word 'db' should not be stemmed")
	}

	// But other words should be stemmed
	stem := stemmer.Stem("running")
	if stem == "running" {
		t.Error("Non-excluded word should be stemmed")
	}
}

func TestStemMinLength(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 5, nil)

	// Words shorter than minLength should not be stemmed
	if stemmer.Stem("run") != "run" {
		t.Error("Word shorter than minLength should not be stemmed")
	}

	// Words meeting minLength should be stemmed
	stem := stemmer.Stem("running")
	if stem == "running" {
		t.Error("Word meeting minLength should be stemmed")
	}
}

func TestPorter2Stemming(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	tests := []struct {
		word     string
		expected string
		message  string
	}{
		{"running", "run", "present participle"},
		{"runs", "run", "plural"},
		{"runner", "runner", "porter2 keeps runner"},
		{"authentication", "authent", "suffix removal"},
		{"authenticate", "authent", "suffix removal"},
		{"database", "databas", "suffix removal"},
		{"searching", "search", "suffix removal"},
		{"function", "function", "no change needed"},
		{"process", "process", "no change needed"},
	}

	for _, test := range tests {
		result := stemmer.Stem(test.word)
		if result != test.expected {
			t.Errorf("%s: Stem(%q) = %q, expected %q",
				test.message, test.word, result, test.expected)
		}
	}
}

func TestStemAll(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	words := []string{"running", "runs", "runner"}
	results := stemmer.StemAll(words)

	if len(results) != len(words) {
		t.Errorf("Expected %d results, got %d", len(words), len(results))
	}

	// At least some should be different from original
	foundDifferent := false
	for _, result := range results {
		for _, word := range words {
			if word != result {
				foundDifferent = true
				break
			}
		}
		if foundDifferent {
			break
		}
	}

	if !foundDifferent {
		t.Error("Some words should be stemmed")
	}
}

func TestStemAndGroup(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	words := []string{"running", "run", "runs", "database", "databases"}
	groups := stemmer.StemAndGroup(words)

	// Should have at least 2 groups (run variants and database variants)
	if len(groups) < 2 {
		t.Errorf("Expected at least 2 groups, got %d", len(groups))
	}

	// Check that grouped words have same stem
	for stem, variations := range groups {
		for _, word := range variations {
			if stemmer.Stem(word) != stem {
				t.Errorf("Word %q in group %q should stem to %q",
					word, stem, stem)
			}
		}
	}
}

func TestGetVariations(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	candidates := []string{
		"run", "running", "runs", "runner",
		"database", "databases", "search", "searching",
	}

	variations := stemmer.GetVariations("running", candidates)

	// Should find "run" and "running" (but maybe not "runner" depending on porter2)
	if len(variations) == 0 {
		t.Error("Should find at least one variation")
	}

	// Check that all variations have same stem as original
	originalStem := stemmer.Stem("running")
	for _, variation := range variations {
		if stemmer.Stem(variation) != originalStem {
			t.Errorf("Variation %q should have same stem as 'running'", variation)
		}
	}
}

func TestNormalizeTerms(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	terms := []string{"running", "runs", "run", "runner"}
	normalized := stemmer.NormalizeTerms(terms)

	// Should have reduced the terms to unique stems
	if len(normalized) >= len(terms) {
		t.Error("Normalization should reduce term count")
	}

	// All terms should be present as keys
	for stem := range normalized {
		// Each stem should map to true
		if !normalized[stem] {
			t.Errorf("Stem %q should map to true", stem)
		}
	}
}

func TestStemmerEnableDisable(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	stemmer.Disable()
	if stemmer.IsEnabled() {
		t.Error("Stemmer should be disabled")
	}

	if stemmer.Stem("running") != "running" {
		t.Error("Stemming should return original when disabled")
	}

	stemmer.Enable()
	if !stemmer.IsEnabled() {
		t.Error("Stemmer should be enabled")
	}

	if stemmer.Stem("running") == "running" {
		t.Error("Stemming should return stem when enabled")
	}
}

func TestSetMinLength(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	err := stemmer.SetMinLength(5)
	if err != nil {
		t.Errorf("SetMinLength(5) failed: %v", err)
	}

	if stemmer.GetMinLength() != 5 {
		t.Error("Min length was not updated")
	}

	// Invalid min length
	err = stemmer.SetMinLength(-1)
	if err == nil {
		t.Error("SetMinLength(-1) should have failed")
	}
}

func TestExclusionManagement(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	// Add exclusion
	stemmer.AddExclusion("api")
	if !stemmer.IsExcluded("api") {
		t.Error("api should be excluded after adding")
	}

	// Should not stem excluded word
	if stemmer.Stem("api") != "api" {
		t.Error("Excluded word should not be stemmed")
	}

	// Remove exclusion
	stemmer.RemoveExclusion("api")
	if stemmer.IsExcluded("api") {
		t.Error("api should not be excluded after removing")
	}

	// Case insensitive
	stemmer.AddExclusion("API")
	if !stemmer.IsExcluded("api") {
		t.Error("Exclusions should be case insensitive")
	}
}

func TestStemmerValidateConfig(t *testing.T) {
	tests := []struct {
		enabled   bool
		algorithm string
		minLength int
		valid     bool
		message   string
	}{
		{true, "porter2", 3, true, "valid config"},
		{true, "none", 0, true, "valid none algorithm"},
		{true, "invalid", 3, false, "invalid algorithm"},
	}

	for _, test := range tests {
		// Note: NewStemmer corrects negative minLength to 0, so we only test via SetMinLength
		stemmer := NewStemmer(test.enabled, test.algorithm, test.minLength, nil)
		err := stemmer.ValidateConfig()

		if test.valid && err != nil {
			t.Errorf("%s: ValidateConfig() should not error, got %v", test.message, err)
		}

		if !test.valid && err == nil {
			t.Errorf("%s: ValidateConfig() should have errored", test.message)
		}
	}
}

func TestNewStemmerFromDict(t *testing.T) {
	dict := DefaultTranslationDictionary()
	stemmer := NewStemmerFromDict(dict)

	if !stemmer.IsEnabled() {
		t.Error("Stemmer should be enabled from dict")
	}

	if stemmer.GetAlgorithm() != "porter2" {
		t.Errorf("Expected porter2 algorithm, got %s", stemmer.GetAlgorithm())
	}

	if stemmer.GetMinLength() != 3 {
		t.Errorf("Expected min length 3, got %d", stemmer.GetMinLength())
	}

	// Check that exclusions from dict are present
	if !stemmer.IsExcluded("api") {
		t.Error("api should be excluded from dict config")
	}

	if !stemmer.IsExcluded("db") {
		t.Error("db should be excluded from dict config")
	}
}

func TestNewStemmerFromNilDict(t *testing.T) {
	stemmer := NewStemmerFromDict(nil)

	if stemmer == nil {
		t.Fatal("NewStemmerFromDict(nil) returned nil")
	}

	// Should have safe defaults
	if stemmer.GetMinLength() != 3 {
		t.Errorf("Should have default min length 3, got %d", stemmer.GetMinLength())
	}

	if stemmer.GetAlgorithm() != "porter2" {
		t.Errorf("Should have default algorithm porter2, got %s", stemmer.GetAlgorithm())
	}
}

func TestAnalyzeStemming(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	words := []string{
		"running", "runs", "run",
		"database", "databases",
		"search", "searching", "searches",
	}

	analysis := stemmer.AnalyzeStemming(words)

	stats := analysis.GetStats()

	if stats["input_words"] != len(words) {
		t.Errorf("Input words should be %d, got %d", len(words), stats["input_words"])
	}

	uniqueStems := stats["unique_stems"].(int)
	if uniqueStems >= len(words) {
		t.Errorf("Stemming should reduce unique terms, got %d from %d", uniqueStems, len(words))
	}

	ratio := stats["compression_ratio"].(float64)
	if ratio >= 1.0 {
		t.Errorf("Compression ratio should be < 1.0, got %.2f", ratio)
	}
}

func TestStemmerChain(t *testing.T) {
	stemmer1 := NewStemmer(true, "porter2", 3, nil)
	stemmer2 := NewStemmer(true, "none", 3, nil) // No-op stemmer

	chain := NewStemmerChain(stemmer1, stemmer2)

	result := chain.Process("running")

	// Should apply stemmer1 (porter2)
	expected := stemmer1.Stem("running")
	if result != expected {
		t.Errorf("Chain process(%q) = %q, expected %q", "running", result, expected)
	}
}

func TestStemmerChainAll(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)
	chain := NewStemmerChain(stemmer)

	words := []string{"running", "database", "searching"}
	results := chain.ProcessAll(words)

	if len(results) != len(words) {
		t.Errorf("Expected %d results, got %d", len(words), len(results))
	}

	for i, word := range words {
		expected := stemmer.Stem(word)
		if results[i] != expected {
			t.Errorf("Chain.ProcessAll: word %d = %q, expected %q", i, results[i], expected)
		}
	}
}

func TestGetExclusions(t *testing.T) {
	exclusions := map[string]bool{"api": true, "db": true}
	stemmer := NewStemmer(true, "porter2", 3, exclusions)

	retrieved := stemmer.GetExclusions()

	if len(retrieved) != len(exclusions) {
		t.Errorf("Expected %d exclusions, got %d", len(exclusions), len(retrieved))
	}

	for key := range exclusions {
		if !retrieved[key] {
			t.Errorf("Exclusion %q not found in retrieved", key)
		}
	}

	// Verify it's a copy, not reference
	retrieved["new"] = true
	if stemmer.IsExcluded("new") {
		t.Error("Modification to retrieved exclusions should not affect stemmer")
	}
}

func TestExclusionCaseInsensitivity(t *testing.T) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	stemmer.AddExclusion("API")
	stemmer.AddExclusion("Database")

	// All variations should match
	if !stemmer.IsExcluded("api") {
		t.Error("api should match API exclusion")
	}

	if !stemmer.IsExcluded("API") {
		t.Error("API should match API exclusion")
	}

	if !stemmer.IsExcluded("database") {
		t.Error("database should match Database exclusion")
	}

	if !stemmer.IsExcluded("DATABASE") {
		t.Error("DATABASE should match Database exclusion")
	}
}

func BenchmarkStem(b *testing.B) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = stemmer.Stem("authentication")
	}
}

func BenchmarkStemAll(b *testing.B) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	words := []string{
		"running", "authentication", "database", "searching",
		"processing", "communication", "function", "variable",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = stemmer.StemAll(words)
	}
}

func BenchmarkStemAndGroup(b *testing.B) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	words := []string{
		"running", "runs", "runner", "authentication", "authenticate",
		"authenticating", "database", "databases", "searching", "searches",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = stemmer.StemAndGroup(words)
	}
}

func BenchmarkAnalyzeStemming(b *testing.B) {
	stemmer := NewStemmer(true, "porter2", 3, nil)

	words := []string{
		"running", "runs", "runner", "authentication", "authenticate",
		"authenticating", "database", "databases", "searching", "searches",
		"process", "processing", "processes", "function", "functions",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = stemmer.AnalyzeStemming(words)
	}
}
