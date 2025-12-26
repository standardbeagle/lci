package mcp

import (
	"fmt"
	"strings"

	"github.com/hbollon/go-edlib"
)

// SymbolTypeResolution contains the result of resolving a symbol type input
type SymbolTypeResolution struct {
	Original  string // Original input from user
	Resolved  string // Resolved canonical type (empty if no match)
	MatchType string // "exact", "alias", "prefix", "fuzzy", "none"
	Warning   string // Warning message for prefix/fuzzy matches
}

// CanonicalSymbolTypes contains all valid symbol types from types.SymbolType
// This must stay in sync with internal/types/types.go SymbolType.String()
var CanonicalSymbolTypes = []string{
	// Core types (matching SymbolType enum order)
	"function", "class", "method", "variable", "constant", "interface", "type",
	// Phase 4 types
	"struct", "module", "namespace",
	// C# specific types
	"property", "event", "delegate", "enum", "record", "operator", "indexer",
	// Kotlin specific types
	"object", "companion", "extension", "annotation",
	// Additional types
	"field", "enum_member",
	// Rust specific types (also aliased to common types for cross-language search)
	"trait", "impl",
	// Constructor
	"constructor",
}

// symbolTypeAliases maps abbreviations and language-specific terms to canonical types
var symbolTypeAliases = map[string]string{
	// Common abbreviations
	"func":  "function",
	"var":   "variable",
	"const": "constant",
	"cls":   "class",
	"meth":  "method",
	"iface": "interface",
	"prop":  "property",
	"ns":    "namespace",
	"mod":   "module",

	// Language-specific (Python)
	"def": "function",

	// Language-specific (Rust)
	"fn": "function",
	// Note: "trait" and "impl" are now first-class types, not aliases

	// Language-specific (Swift)
	"protocol": "interface",

	// Language-specific (JavaScript/Kotlin/Scala)
	"let": "variable",
	"val": "variable",

	// Plural forms (common mistakes)
	"functions":  "function",
	"variables":  "variable",
	"classes":    "class",
	"methods":    "method",
	"interfaces": "interface",
	"constants":  "constant",
	"structs":    "struct",
	"enums":      "enum",
	"types":      "type",
	"fields":     "field",
	"properties": "property",
}

// SymbolTypeResolver handles symbol type validation and resolution
type SymbolTypeResolver struct {
	validTypes   map[string]bool
	aliases      map[string]string
	canonicalSet []string
}

// NewSymbolTypeResolver creates a new resolver with all canonical types and aliases
func NewSymbolTypeResolver() *SymbolTypeResolver {
	validTypes := make(map[string]bool)
	for _, t := range CanonicalSymbolTypes {
		validTypes[t] = true
	}

	return &SymbolTypeResolver{
		validTypes:   validTypes,
		aliases:      symbolTypeAliases,
		canonicalSet: CanonicalSymbolTypes,
	}
}

// Resolve attempts to resolve a single symbol type input to a canonical type
// Resolution priority: exact match > alias > prefix > fuzzy
func (r *SymbolTypeResolver) Resolve(input string) SymbolTypeResolution {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" {
		return SymbolTypeResolution{
			Original:  input,
			MatchType: "none",
		}
	}

	// Priority 1: Exact match (case-insensitive)
	if r.validTypes[normalized] {
		return SymbolTypeResolution{
			Original:  input,
			Resolved:  normalized,
			MatchType: "exact",
		}
	}

	// Priority 2: Alias match
	if canonical, ok := r.aliases[normalized]; ok {
		return SymbolTypeResolution{
			Original:  input,
			Resolved:  canonical,
			MatchType: "alias",
		}
	}

	// Priority 3: Prefix match (min 3 chars to avoid ambiguity)
	if len(normalized) >= 3 {
		for _, canonical := range r.canonicalSet {
			if strings.HasPrefix(canonical, normalized) {
				return SymbolTypeResolution{
					Original:  input,
					Resolved:  canonical,
					MatchType: "prefix",
					Warning:   fmt.Sprintf("'%s' interpreted as '%s' (prefix match)", input, canonical),
				}
			}
		}
	}

	// Priority 4: Fuzzy match (Levenshtein distance <= 2)
	bestMatch, distance := r.findClosestMatch(normalized)
	if distance <= 2 && distance > 0 {
		return SymbolTypeResolution{
			Original:  input,
			Resolved:  bestMatch,
			MatchType: "fuzzy",
			Warning:   fmt.Sprintf("'%s' interpreted as '%s' (did you mean '%s'?)", input, bestMatch, bestMatch),
		}
	}

	// No match found
	return SymbolTypeResolution{
		Original:  input,
		Resolved:  "",
		MatchType: "none",
		Warning:   fmt.Sprintf("unknown symbol type '%s'", input),
	}
}

// findClosestMatch finds the canonical type with the smallest Levenshtein distance
func (r *SymbolTypeResolver) findClosestMatch(input string) (string, int) {
	bestMatch := ""
	bestDistance := 1000 // Large initial value

	for _, canonical := range r.canonicalSet {
		distance := edlib.LevenshteinDistance(input, canonical)
		if distance < bestDistance {
			bestDistance = distance
			bestMatch = canonical
		}
	}

	return bestMatch, bestDistance
}

// ResolveAll resolves a comma-separated list of symbol types
// Returns the resolved types and any warnings generated
func (r *SymbolTypeResolver) ResolveAll(input string) ([]string, []string) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}

	var resolved []string
	var warnings []string
	seen := make(map[string]bool) // Deduplicate resolved types

	for _, item := range strings.Split(input, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}

		resolution := r.Resolve(trimmed)
		if resolution.Resolved != "" && !seen[resolution.Resolved] {
			resolved = append(resolved, resolution.Resolved)
			seen[resolution.Resolved] = true
		}
		if resolution.Warning != "" {
			warnings = append(warnings, resolution.Warning)
		}
	}

	return resolved, warnings
}

// GetValidTypesDescription returns a formatted description of valid types for tool documentation
func GetValidTypesDescription() string {
	return fmt.Sprintf(`Symbol types to filter results (comma-separated).
Valid types: %s.
Aliases: func->function, var->variable, const->constant, cls->class, meth->method, iface->interface, def->function (Python), fn->function (Rust), trait->interface (Rust).
Prefix and fuzzy matching supported with warnings.
Examples: "function,class", "func,cls", "method"`, strings.Join(CanonicalSymbolTypes, ", "))
}
