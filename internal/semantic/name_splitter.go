package semantic

import (
	"strings"
	"sync"
	"unicode"
)

// NameSplitter handles splitting symbol names into constituent words
// Uses a two-pass approach: first detect separator types, then split efficiently
// Supports: camelCase, snake_case, kebab-case, PascalCase, SCREAMING_SNAKE_CASE, etc.
//
// Thread-safe: Cache uses sync.Map for concurrent access with LRU eviction
type NameSplitter struct {
	cache sync.Map // Cache for expensive split operations (28.22% of CPU time)

	// Simple LRU tracking to prevent unbounded memory growth
	cacheKeys []string   // Track insertion order for LRU
	maxSize   int        // Maximum cache size before eviction
	mu        sync.Mutex // Protect cacheKeys operations
}

// Default cache size limits
const (
	DefaultCacheSize = 1000 // Maximum number of cached split results
)

// NewNameSplitter creates a new name splitter with cache
func NewNameSplitter() *NameSplitter {
	return NewNameSplitterWithSize(DefaultCacheSize)
}

// NewNameSplitterWithSize creates a new name splitter with custom cache size
func NewNameSplitterWithSize(cacheSize int) *NameSplitter {
	return &NameSplitter{
		cacheKeys: make([]string, 0, cacheSize),
		maxSize:   cacheSize,
	}
}

// SeparatorType represents the type of separators found in a name
type SeparatorType uint8

const (
	SepNone       SeparatorType = 0
	SepUnderscore SeparatorType = 1 << iota
	SepHyphen
	SepDot
	SepSlash
	SepCamelCase
	SepPascalCase
	SepDigits
)

// detectSeparators performs first pass to identify separator types present
func (ns *NameSplitter) detectSeparators(name string) SeparatorType {
	if len(name) == 0 {
		return SepNone
	}

	var seps SeparatorType
	runes := []rune(name)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Check for explicit separators
		switch ch {
		case '_':
			seps |= SepUnderscore
		case '-':
			seps |= SepHyphen
		case '.':
			seps |= SepDot
		case '/':
			seps |= SepSlash
		}

		// Check for case transitions (camelCase/PascalCase)
		if i > 0 {
			prev := runes[i-1]

			// lowercase to uppercase transition (camelCase)
			if unicode.IsLower(prev) && unicode.IsUpper(ch) {
				seps |= SepCamelCase
			}

			// Check for uppercase followed by lowercase (PascalCase/acronyms)
			if unicode.IsUpper(prev) && unicode.IsLower(ch) && i > 1 {
				// Look back to see if we're in an acronym
				prevPrev := runes[i-2]
				if unicode.IsUpper(prevPrev) {
					seps |= SepPascalCase
				}
			}

			// Check for letter-digit transitions
			if (unicode.IsLetter(prev) && unicode.IsDigit(ch)) ||
				(unicode.IsDigit(prev) && unicode.IsLetter(ch)) {
				seps |= SepDigits
			}
		}
	}

	return seps
}

// Split splits a symbol name into constituent words
// Uses two-pass approach: detect separators, then split accordingly
func (ns *NameSplitter) Split(name string) []string {
	if name == "" {
		return []string{}
	}

	// Check cache first (massive performance win for repeated queries)
	if cached, ok := ns.cache.Load(name); ok {
		return cached.([]string)
	}

	// First pass: detect what types of separators we have
	seps := ns.detectSeparators(name)
	if seps == SepNone {
		// No separators, return as-is (lowercase)
		return []string{strings.ToLower(name)}
	}

	// Second pass: split based on detected separators
	runes := []rune(name)

	// Use local buffers for thread-safety
	wordBuffer := make([]rune, 0, 64)
	words := make([]string, 0, 8)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Check for explicit separator characters
		if ch == '_' || ch == '-' || ch == '.' || ch == '/' {
			// Flush current word if we have one
			if len(wordBuffer) > 0 {
				words = append(words, strings.ToLower(string(wordBuffer)))
				wordBuffer = wordBuffer[:0]
			}
			continue // Skip the separator itself
		}

		// Handle case transitions if detected
		if i > 0 && (seps&(SepCamelCase|SepPascalCase) != 0) {
			prev := runes[i-1]

			// lowercase to uppercase transition (camelCase)
			if unicode.IsLower(prev) && unicode.IsUpper(ch) {
				// Start new word
				if len(wordBuffer) > 0 {
					words = append(words, strings.ToLower(string(wordBuffer)))
					wordBuffer = wordBuffer[:0]
				}
			}

			// Uppercase followed by lowercase (HTTPServer -> HTTP Server)
			if i > 1 && unicode.IsUpper(prev) && unicode.IsLower(ch) {
				prevPrev := runes[i-2]
				if unicode.IsUpper(prevPrev) {
					// We're at the end of an acronym
					// Move the last character to start of new word
					if len(wordBuffer) > 0 {
						// Remove last char from buffer
						lastChar := wordBuffer[len(wordBuffer)-1]
						wordBuffer = wordBuffer[:len(wordBuffer)-1]

						// Save current word if non-empty
						if len(wordBuffer) > 0 {
							words = append(words, strings.ToLower(string(wordBuffer)))
						}

						// Start new word with the uppercase letter
						wordBuffer = wordBuffer[:0]
						wordBuffer = append(wordBuffer, lastChar)
					}
				}
			}
		}

		// Handle digit transitions if detected
		if i > 0 && (seps&SepDigits != 0) {
			prev := runes[i-1]

			// Letter to digit or digit to letter transition
			if (unicode.IsLetter(prev) && unicode.IsDigit(ch)) ||
				(unicode.IsDigit(prev) && unicode.IsLetter(ch)) {
				// Start new word
				if len(wordBuffer) > 0 {
					words = append(words, strings.ToLower(string(wordBuffer)))
					wordBuffer = wordBuffer[:0]
				}
			}
		}

		// Add character to current word
		wordBuffer = append(wordBuffer, ch)
	}

	// Flush final word
	if len(wordBuffer) > 0 {
		words = append(words, strings.ToLower(string(wordBuffer)))
	}

	// Cache the result with LRU management
	ns.cacheWithLRU(name, words)

	return words
}

// cacheWithLRU stores result in cache with LRU eviction
func (ns *NameSplitter) cacheWithLRU(name string, words []string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Check if we need to evict
	if len(ns.cacheKeys) >= ns.maxSize {
		// Evict the oldest entry (simple LRU)
		if len(ns.cacheKeys) > 0 {
			oldestKey := ns.cacheKeys[0]
			ns.cache.Delete(oldestKey)
			// Remove from keys tracking
			ns.cacheKeys = ns.cacheKeys[1:]
		}
	}

	// Store new entry
	ns.cache.Store(name, words)
	ns.cacheKeys = append(ns.cacheKeys, name)
}

// normalizeSeparators is kept for backward compatibility but now just calls Split
func (ns *NameSplitter) normalizeSeparators(name string) string {
	words := ns.Split(name)
	return strings.Join(words, " ")
}

// splitCamelCase is kept for backward compatibility but now just calls Split
func (ns *NameSplitter) splitCamelCase(name string) string {
	words := ns.Split(name)
	return strings.Join(words, " ")
}

// SplitToSet splits a name and returns unique words as a set
func (ns *NameSplitter) SplitToSet(name string) map[string]bool {
	words := ns.Split(name)
	set := make(map[string]bool, len(words))
	for _, word := range words {
		if word != "" {
			set[word] = true
		}
	}
	return set
}
