package mcp

// Default values for search operations
const (
	// Search result defaults
	SearchDefaultMax = 100 // Conservative default for detailed results
	// Rationale: Balances comprehensive results with memory usage
	// and response size. 100 results typically fit within
	// AI context windows while providing good coverage.

	SearchDefaultContextLines = 5 // Good balance for AI understanding
	// Rationale: 5 lines above/below provides enough context
	// for understanding code purpose without overwhelming
	// the response. Based on typical function/method sizes.

	// Grep search defaults (optimized for speed)
	GrepDefaultMax = 500 // Higher default for fast grep mode
	// Rationale: Grep mode prioritizes speed over detail,
	// so we can handle more results. 500 provides broad
	// coverage for exploratory searches.

	GrepDefaultContextLines = 3 // Less context for speed
	// Rationale: Minimal context (3 lines) reduces processing
	// time and response size while still providing basic
	// understanding of match location.

	// Index comparison defaults
	IndexCompareDefaultMaxLines = 3 // Default context lines
	// Rationale: For comparing implementations, 3 lines
	// provides enough context to see differences without
	// cluttering the comparison view.

	// Pagination constants
	PaginationDefaultContextLines = 3 // Default context for pagination
	// Rationale: Matches grep context for consistency
	// and keeps paginated results compact.

	PaginationBaseTokens = 50 // Minimal result token count
	// Rationale: Based on average token count for a
	// basic search result (file path + line + match).
	// Provides baseline for token estimation.

	PaginationMetadataTokens = 100 // Reserve for metadata
	// Rationale: Covers JSON structure, query info,
	// pagination details, and response wrapper.
	// Ensures metadata doesn't exceed token budget.
)

// Semantic scoring constants
const (
	// MaxSemanticScoringCandidates is the maximum number of candidates allowed for semantic scoring
	MaxSemanticScoringCandidates = 1000
	// Rationale: Prevents excessive memory usage and computation time
	// while supporting reasonable batch sizes for code search.

	// DefaultSemanticCacheSize is the default size for the semantic scorer query cache
	DefaultSemanticCacheSize = 1000
	// Rationale: Balances memory usage with cache hit rate for
	// typical development sessions with repeated queries.

	// DefaultFuzzyThreshold is the default similarity threshold for fuzzy matching
	DefaultFuzzyThreshold = 0.7
	// Rationale: 0.7 Jaro-Winkler similarity captures most typos
	// and minor variations while avoiding false positives.

	// DefaultStemmingMinLength is the minimum word length for stemming
	DefaultStemmingMinLength = 3
	// Rationale: Words shorter than 3 chars are often abbreviations
	// or keywords that shouldn't be stemmed.

	// DefaultStemmerAlgorithm is the default stemming algorithm
	DefaultStemmerAlgorithm = "porter2"
	// Rationale: Porter2 is the most widely used and tested
	// English language stemmer with good accuracy.

	// DefaultFuzzyAlgorithm is the default fuzzy matching algorithm
	DefaultFuzzyAlgorithm = "jaro-winkler"
	// Rationale: Jaro-Winkler performs well for short strings
	// like function and variable names in code.
)

// Codebase intelligence constants
const (
	// DefaultCodebaseIntelligenceTier is the default tier for overview mode
	DefaultCodebaseIntelligenceTier = 1
	// Rationale: Tier 1 provides essential overview (79.8% context reduction)
	// suitable for initial exploration.

	// DefaultConfidenceThreshold is the default confidence threshold for analysis
	DefaultConfidenceThreshold = 0.7
	// Rationale: 0.7 confidence balances precision and recall,
	// filtering out low-confidence results while keeping useful ones.

	// DefaultGranularity is the default granularity for analysis
	DefaultGranularity = "module"
	// Rationale: Module-level analysis provides good abstraction
	// for understanding code organization without excessive detail.

	// DefaultIndexingTimeout is the default timeout for waiting on indexing completion
	DefaultIndexingTimeout = 120 // seconds
	// Rationale: 120 seconds allows time for indexing very large projects with -p questions
	// and complex analysis. Supports production projects with thousands of files.
	// Can be configured via config.Performance.IndexingTimeoutSec.
)
