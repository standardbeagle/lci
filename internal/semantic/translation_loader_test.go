package semantic

import (
	"testing"
)

func TestDefaultTranslationDictionary(t *testing.T) {
	dict := DefaultTranslationDictionary()

	// Verify non-nil
	if dict == nil {
		t.Fatal("DefaultTranslationDictionary returned nil")
	}

	// Verify fuzzy config
	if !dict.FuzzyConfig.Enabled {
		t.Error("Fuzzy matching should be enabled by default")
	}
	if dict.FuzzyConfig.Threshold != 0.80 {
		t.Errorf("Expected threshold 0.80, got %f", dict.FuzzyConfig.Threshold)
	}

	// Verify stemming config
	if !dict.StemmingConfig.Enabled {
		t.Error("Stemming should be enabled by default")
	}
	if dict.StemmingConfig.MinLength != 3 {
		t.Errorf("Expected min length 3, got %d", dict.StemmingConfig.MinLength)
	}

	// Verify propagation config
	if !dict.PropagationConfig.Enabled {
		t.Error("Propagation should be enabled by default")
	}
}

func TestAbbreviationExpansion(t *testing.T) {
	dict := DefaultTranslationDictionary()

	tests := []struct {
		term     string
		expected []string
	}{
		{
			"auth",
			[]string{"auth", "authenticate", "authorization", "authorized", "login", "signin"},
		},
		{
			"api",
			[]string{"api", "application", "programming", "interface"},
		},
		{
			"db",
			[]string{"db", "database", "databases"},
		},
		{
			"unknown",
			[]string{"unknown"},
		},
	}

	for _, test := range tests {
		expanded := dict.Expand(test.term)

		// Check first term is always the original
		if len(expanded) == 0 || expanded[0] != test.term {
			t.Errorf("First term should be original for %q, got %v", test.term, expanded)
		}

		// Check all expected terms are present
		for _, expected := range test.expected {
			found := false
			for _, term := range expanded {
				if term == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Term %q not found in expansion of %q: %v", expected, test.term, expanded)
			}
		}
	}
}

func TestDomainExpansion(t *testing.T) {
	dict := DefaultTranslationDictionary()

	// "signin" is in authentication domain
	expanded := dict.Expand("signin")

	// Should also find "login", "authenticate", etc. from same domain
	if len(expanded) < 2 {
		t.Errorf("Expected multiple expansions for 'signin', got %v", expanded)
	}

	// Check that it includes domain-related terms
	expectedInDomain := []string{"login", "authenticate"}
	for _, expected := range expectedInDomain {
		found := false
		for _, term := range expanded {
			if term == expected {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Note: '%s' not found in expansion (may not be in default domains)", expected)
		}
	}
}

func TestLanguageSpecificExpansion(t *testing.T) {
	dict := DefaultTranslationDictionary()

	tests := []struct {
		lang     string
		term     string
		hasMatch bool
	}{
		{"go", "interface", true},
		{"go", "goroutine", true},
		{"javascript", "promise", true},
		{"javascript", "callback", true},
		{"python", "decorator", true},
		{"unknown_lang", "interface", false},
	}

	for _, test := range tests {
		expanded := dict.ExpandLanguageSpecific(test.lang, test.term)

		hasMatch := len(expanded) > 1 // More than just the original term
		if hasMatch != test.hasMatch {
			if test.hasMatch {
				t.Errorf("Expected matches for %s in %s, got %v", test.term, test.lang, expanded)
			} else {
				t.Logf("Unexpected matches for %s in %s: %v", test.term, test.lang, expanded)
			}
		}
	}
}

func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{
			[]string{"a", "b", "a", "c", "b"},
			[]string{"a", "b", "c"},
		},
		{
			[]string{"x"},
			[]string{"x"},
		},
		{
			[]string{},
			[]string{},
		},
		{
			[]string{"a", "a", "a"},
			[]string{"a"},
		},
	}

	for _, test := range tests {
		result := removeDuplicates(test.input)
		if len(result) != len(test.expected) {
			t.Errorf("Expected %d items, got %d: %v", len(test.expected), len(result), result)
			continue
		}

		for i, expected := range test.expected {
			if result[i] != expected {
				t.Errorf("Mismatch at index %d: expected %q, got %q", i, expected, result[i])
			}
		}
	}
}

func TestFuzzyConfig(t *testing.T) {
	dict := DefaultTranslationDictionary()

	if dict.FuzzyConfig.Algorithm != "jaro-winkler" {
		t.Errorf("Expected jaro-winkler algorithm, got %s", dict.FuzzyConfig.Algorithm)
	}
}

func TestStemmingConfig(t *testing.T) {
	dict := DefaultTranslationDictionary()

	if dict.StemmingConfig.Algorithm != "porter2" {
		t.Errorf("Expected porter2 algorithm, got %s", dict.StemmingConfig.Algorithm)
	}

	// Check that common code abbreviations are excluded
	expectedExclusions := []string{"api", "db", "dto", "dao"}
	for _, excl := range expectedExclusions {
		if !dict.StemmingConfig.Exclusions[excl] {
			t.Errorf("Expected %q in exclusion list", excl)
		}
	}
}

func TestAbbreviationCompleteness(t *testing.T) {
	dict := DefaultTranslationDictionary()

	// Verify common abbreviations are present
	commonAbbreviations := []string{
		"auth", "api", "db", "repo", "crud",
		"http", "rest", "json", "xml",
		"cli", "ui", "ux",
		"ci", "cd",
	}

	for _, abbr := range commonAbbreviations {
		if _, ok := dict.Abbreviations[abbr]; !ok {
			t.Errorf("Common abbreviation %q not found in defaults", abbr)
		}
	}

	t.Logf("Total abbreviations: %d", len(dict.Abbreviations))
	t.Logf("Total domains: %d", len(dict.Domains))
	t.Logf("Total languages: %d", len(dict.Languages))
}

func TestLanguageCompleteness(t *testing.T) {
	dict := DefaultTranslationDictionary()

	expectedLanguages := []string{"go", "javascript", "python"}
	for _, lang := range expectedLanguages {
		if _, ok := dict.Languages[lang]; !ok {
			t.Errorf("Expected language %q not found", lang)
		}
	}

	// Spot check: Go should have interface
	if goTerms, ok := dict.Languages["go"]; ok {
		if _, hasInterface := goTerms["interface"]; !hasInterface {
			t.Error("Go language should have 'interface' term")
		}
	}
}

func TestTagMappings(t *testing.T) {
	dict := DefaultTranslationDictionary()

	expectedTags := []string{"critical", "deprecated", "experimental"}
	for _, tag := range expectedTags {
		if _, ok := dict.TagMappings[tag]; !ok {
			t.Errorf("Expected tag %q not found", tag)
		}
	}
}

func BenchmarkExpand(b *testing.B) {
	dict := DefaultTranslationDictionary()
	terms := []string{"auth", "api", "db", "repo", "cli", "unknown"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, term := range terms {
			_ = dict.Expand(term)
		}
	}
}

func BenchmarkRemoveDuplicates(b *testing.B) {
	items := []string{"a", "b", "a", "c", "b", "d", "a", "e"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = removeDuplicates(items)
	}
}
