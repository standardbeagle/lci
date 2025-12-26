// Package semantic provides advanced semantic search capabilities for code analysis.
//
// The semantic package implements a multi-layered matching system that progressively
// falls back through increasingly fuzzy matching strategies to find the best matches
// for a given query. This is particularly useful for code search where developers
// may not remember exact function names or may make typos.
//
// # Matching Layers
//
// The package provides six layers of matching, each with decreasing precision:
//
//  1. Exact Match - Direct string comparison (case-insensitive)
//  2. Annotation Match - Matches against @lci: comment annotations
//  3. Fuzzy Match - Uses Jaro-Winkler similarity for typo tolerance
//  4. Stemming Match - Reduces words to their root forms
//  5. Abbreviation Match - Expands common programming abbreviations
//  6. Name Split Match - Splits camelCase and snake_case identifiers
//
// # Core Components
//
// SemanticScorer: The main entry point that coordinates all matching layers
// and returns ranked results based on match quality.
//
// FuzzyMatcher: Implements fuzzy string matching using configurable algorithms
// (currently supports Jaro-Winkler and Levenshtein distance).
//
// Stemmer: Reduces words to their root forms using the Porter2 algorithm,
// enabling matches between different word forms (e.g., "validate" and "validation").
//
// NameSplitter: Intelligently splits compound identifiers into component words,
// supporting camelCase, PascalCase, snake_case, and kebab-case conventions.
//
// # Usage Example
//
//	// Create components
//	splitter := semantic.NewNameSplitter()
//	stemmer := semantic.NewStemmer(true, "porter2", 3, nil)
//	fuzzer := semantic.NewFuzzyMatcher(true, 0.7, "jaro-winkler")
//	dict := semantic.DefaultTranslationDictionary()
//
//	// Create scorer
//	scorer := semantic.NewSemanticScorer(splitter, stemmer, fuzzer, dict, nil)
//
//	// Score candidates
//	candidates := []string{"getUserName", "setUserName", "userName"}
//	results := scorer.ScoreMultiple("getUserNme", candidates)
//
// # Performance Considerations
//
// The semantic scorer uses an LRU cache to store normalized queries, reducing
// repeated computation costs. The cache size is configurable but defaults to
// 1000 entries.
//
// For large candidate sets (>1000 items), consider batching or pre-filtering
// to maintain reasonable performance.
package semantic