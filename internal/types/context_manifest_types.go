package types

import (
	"encoding/json"
	"time"
)

// ContextManifest is a compact, serializable representation of code context
// for efficient transfer between AI agent sessions. Stores symbol references,
// not source code - hydration happens on demand via LCI's index.
//
// Design principle: LCI handles structure (compact JSON), agents handle semantics (free-form notes).
type ContextManifest struct {
	// Task description - free-form text for agents
	Task string `json:"t,omitempty"`

	// Metadata
	Created     time.Time `json:"c,omitempty"` // Creation timestamp
	Version     string    `json:"v,omitempty"` // Manifest format version
	ProjectRoot string    `json:"p,omitempty"` // Project root path for relative file paths

	// Symbol references - the core data
	Refs []ContextRef `json:"r"`

	// Statistics for display
	Stats ManifestStats `json:"s,omitempty"`
}

// ContextRef is a compact reference to code with optional expansion directives.
// Stores location + metadata, not source code.
type ContextRef struct {
	// Location (at least one required: F+S, F+L, or just F)
	F string     `json:"f"`           // File path (required)
	S string     `json:"s,omitempty"` // Symbol name (optional - can use line range instead)
	L *LineRange `json:"l,omitempty"` // Line range (optional - can use symbol instead)

	// Expansion directives - what related code to fetch on hydration
	// Examples: "callers", "callees:2", "implementations", "tests"
	X []string `json:"x,omitempty"`

	// Metadata for agent-to-agent communication (LCI passes through, doesn't interpret)
	Role string `json:"role,omitempty"` // Semantic hint: "modify", "contract", "pattern", "boundary"
	Note string `json:"n,omitempty"`    // Free-form architect annotation

	// Internal tracking (not serialized to file, used during hydration)
	ObjectID string `json:"-"` // LCI internal object ID (populated during save)
}

// LineRange specifies a range of lines in a file (1-indexed, inclusive)
type LineRange struct {
	Start int `json:"s"` // Start line (1-indexed)
	End   int `json:"e"` // End line (1-indexed, inclusive)
}

// ManifestStats provides statistics about the manifest
type ManifestStats struct {
	RefCount      int            `json:"rc"`           // Number of refs
	TotalLines    int            `json:"tl,omitempty"` // Total lines referenced (approximate)
	FileCount     int            `json:"fc,omitempty"` // Unique files
	RoleBreakdown map[string]int `json:"rb,omitempty"` // Role → count
	SizeBytes     int            `json:"sb,omitempty"` // Manifest file size in bytes
}

// HydratedContext is the expanded context with full source code and relationships.
// This is what agents receive after loading a manifest.
type HydratedContext struct {
	// Task from manifest
	Task string `json:"task,omitempty"`

	// Hydrated references with source code
	Refs []HydratedRef `json:"refs"`

	// Hydration statistics
	Stats HydrationStats `json:"stats"`

	// Warnings (e.g., missing symbols, truncation)
	Warnings []string `json:"warnings,omitempty"`
}

// HydratedRef is a single reference with resolved source code and expanded relationships
type HydratedRef struct {
	// Location
	File   string    `json:"file"`
	Symbol string    `json:"symbol,omitempty"`
	Lines  LineRange `json:"lines"`

	// Metadata from manifest (passed through from ContextRef)
	Role string `json:"role,omitempty"`
	Note string `json:"note,omitempty"`

	// Hydrated content
	Source string `json:"source"` // Actual source code

	// Symbol metadata (when available)
	SymbolType string `json:"symbol_type,omitempty"` // "function", "class", "method", etc.
	Signature  string `json:"signature,omitempty"`   // Function/method signature

	// Expanded relationships (based on directives in X)
	Expanded map[string][]HydratedRef `json:"expanded,omitempty"`

	// Context quality indicators
	IsExported  bool `json:"is_exported,omitempty"`
	IsGenerated bool `json:"is_generated,omitempty"` // Generated code (warn agent)

	// Purity analysis (for callees/dependencies)
	Purity     *PurityInfo `json:"purity,omitempty"`      // Purity info for this symbol
	IsExternal bool        `json:"is_external,omitempty"` // True if external dependency (not in codebase)
}

// PurityInfo contains purity analysis for a function
type PurityInfo struct {
	IsPure      bool     `json:"is_pure"`                 // True if function has no side effects
	PurityLevel string   `json:"purity_level,omitempty"`  // "Pure", "InternallyPure", "ObjectState", "ModuleGlobal", "ExternalDependency"
	Categories  []string `json:"categories,omitempty"`    // Side effect categories (e.g., "io", "param_write")
	PurityScore float64  `json:"purity_score,omitempty"`  // 0.0-1.0
	Reasons     []string `json:"reasons,omitempty"`       // Why it's impure
}

// HydrationStats provides statistics about the hydration process
type HydrationStats struct {
	RefsLoaded        int  `json:"refs_loaded"`         // Number of refs from manifest
	SymbolsHydrated   int  `json:"symbols_hydrated"`    // Total symbols (including expansions)
	TokensApprox      int  `json:"tokens_approx"`       // Approximate token count
	ExpansionsApplied int  `json:"expansions_applied"`  // Number of expansions executed
	Truncated         bool `json:"truncated,omitempty"` // Whether results were truncated
}

// SaveParams are parameters for the context save operation
type SaveParams struct {
	// What to save
	Refs []ContextRef `json:"refs,omitempty"` // References to save

	// Where to save
	ToFile   string `json:"to_file,omitempty"`   // File path (relative to project root)
	ToString bool   `json:"to_string,omitempty"` // Return as JSON string instead of writing

	// How to save
	Append bool   `json:"append,omitempty"` // Merge with existing manifest (default: false)
	Task   string `json:"task,omitempty"`   // Task description/directive
}

// SaveResponse is the response from a save operation
type SaveResponse struct {
	Saved     string        `json:"saved,omitempty"`    // File path if to_file was used
	Manifest  string        `json:"manifest,omitempty"` // JSON string if to_string was true
	Stats     ManifestStats `json:"stats"`
	RefCount  int           `json:"ref_count"`  // Number of refs saved
	FileCount int           `json:"file_count"` // Number of unique files
}

// LoadParams are parameters for the context load operation
type LoadParams struct {
	// Where to load from
	FromFile   string `json:"from_file,omitempty"`   // File path (relative to project root)
	FromString string `json:"from_string,omitempty"` // Inline JSON string

	// What to include
	Filter  []string `json:"filter,omitempty"`  // Only include these roles
	Exclude []string `json:"exclude,omitempty"` // Exclude these roles

	// Output control
	Format    string `json:"format,omitempty"`     // "full" (default), "signatures", "outline"
	MaxTokens int    `json:"max_tokens,omitempty"` // Approximate token limit (0 = no limit)
}

// ExpansionDirective represents a parsed expansion directive
type ExpansionDirective struct {
	Type  string // "callers", "callees", "implementations", etc.
	Depth int    // For "callers:2", depth is 2. Default is 1.
}

// ParseExpansionDirective parses a directive string like "callers:2" into structured form
func ParseExpansionDirective(directive string) ExpansionDirective {
	// Will implement in expansion engine
	return ExpansionDirective{Type: directive, Depth: 1}
}

// Validate checks if the manifest is valid
func (m *ContextManifest) Validate() error {
	if len(m.Refs) == 0 {
		return nil // Empty manifest is valid
	}

	for i, ref := range m.Refs {
		if ref.F == "" {
			return &ValidationError{
				Field:   "refs[" + string(rune(i)) + "].f",
				Message: "file path is required",
			}
		}

		// At least one of S or L must be present
		if ref.S == "" && ref.L == nil {
			return &ValidationError{
				Field:   "refs[" + string(rune(i)) + "]",
				Message: "either symbol name (s) or line range (l) is required",
			}
		}

		// If line range is present, validate it
		if ref.L != nil {
			if ref.L.Start <= 0 || ref.L.End <= 0 {
				return &ValidationError{
					Field:   "refs[" + string(rune(i)) + "].l",
					Message: "line numbers must be positive (1-indexed)",
				}
			}
			if ref.L.Start > ref.L.End {
				return &ValidationError{
					Field:   "refs[" + string(rune(i)) + "].l",
					Message: "start line must be <= end line",
				}
			}
		}
	}

	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error on field '" + e.Field + "': " + e.Message
}

// ComputeStats computes statistics for the manifest
func (m *ContextManifest) ComputeStats() ManifestStats {
	stats := ManifestStats{
		RefCount:      len(m.Refs),
		RoleBreakdown: make(map[string]int),
	}

	// Track unique files
	fileSet := make(map[string]struct{})
	totalLines := 0

	for _, ref := range m.Refs {
		fileSet[ref.F] = struct{}{}

		// Count lines
		if ref.L != nil {
			totalLines += (ref.L.End - ref.L.Start + 1)
		} else {
			totalLines += 1 // Assume 1 line if no range specified
		}

		// Count roles
		if ref.Role != "" {
			stats.RoleBreakdown[ref.Role]++
		}
	}

	stats.FileCount = len(fileSet)
	stats.TotalLines = totalLines

	return stats
}

// MarshalJSON implements custom JSON marshaling with version
func (m *ContextManifest) MarshalJSON() ([]byte, error) {
	type Alias ContextManifest
	m.Version = "1.0" // Set version on marshal
	m.Stats = m.ComputeStats()
	return json.Marshal((*Alias)(m))
}

// FormatType represents the output format for hydrated context
type FormatType string

const (
	FormatFull       FormatType = "full"       // Full source code
	FormatSignatures FormatType = "signatures" // Just function/method signatures
	FormatOutline    FormatType = "outline"    // Just symbol names and locations
)

// IsValid checks if the format type is valid
func (f FormatType) IsValid() bool {
	switch f {
	case FormatFull, FormatSignatures, FormatOutline:
		return true
	default:
		return false
	}
}
