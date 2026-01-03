package config

import (
	"fmt"
	"os"
	"runtime"
	"github.com/standardbeagle/lci/internal/types"
)

// SearchRankingScoreConstants defines scoring constants for search ranking configuration
// These values are used as defaults in both code and configuration parsing
const (
	DefaultCodeFileBoost    = 50.0
	DefaultDocFilePenalty   = -20.0
	DefaultConfigFileBoost  = 10.0
	DefaultNonSymbolPenalty = -30.0
	RequireSymbolPenalty    = -1000.0
)

type Config struct {
	Version              int
	Project              Project
	Index                Index
	Performance          Performance
	Semantic             Semantic
	SemanticScoring      SemanticScoring
	Search               Search
	FeatureFlags         FeatureFlags
	Include              []string
	Exclude              []string
	PropagationConfigDir string // Directory for propagation configuration files
}

type Project struct {
	Root string
	Name string
}

type Index struct {
	MaxFileSize      int64
	MaxTotalSizeMB   int64
	MaxFileCount     int
	FollowSymlinks   bool
	SmartSizeControl bool
	PriorityMode     string // "recent", "small", "important"
	RespectGitignore bool   // Process .gitignore files for additional exclusions
	WatchMode        bool   // Enable file system watching for automatic reindexing
	WatchDebounceMs  int    // Debounce time for file change events
}

type Performance struct {
	MaxMemoryMB         int // Maximum memory usage in MB
	MaxGoroutines       int // Maximum number of goroutines for indexing
	DebounceMs          int // Debounce time in milliseconds for file change events
	ParallelFileWorkers int // 0 = auto-detect (NumCPU)
	IndexingTimeoutSec  int // Timeout for indexing operations in seconds (default: 120)
	// Use this to configure how long MCP tools wait for indexing to complete.
	// Increase this value for very large codebases (10000+ files) that may take
	// longer to index, especially when using -p questions or complex analysis.
	// Default: 120 seconds. Can be set via config file: .lci.kdl

	StartupDelayMs int // Delay before auto-indexing starts (default: 1500ms)
	// This delay allows the UI (e.g., Claude Code) to become responsive before
	// CPU-intensive indexing begins. Set to 0 to disable the delay.
}

type Semantic struct {
	BatchSize     int // Batch size for semantic index processing
	ChannelSize   int // Channel buffer size for updates
	MinStemLength int // Minimum word length for stemming
	CacheSize     int // NameSplitter cache size
}

type SemanticScoring struct {
	// Individual layer weights
	ExactWeight        float64 // Weight for exact matches
	SubstringWeight    float64 // Weight for substring matches
	AnnotationWeight   float64 // Weight for annotation matches
	FuzzyWeight        float64 // Weight for fuzzy matches
	StemmingWeight     float64 // Weight for stemming matches
	NameSplitWeight    float64 // Weight for name splitting matches
	AbbreviationWeight float64 // Weight for abbreviation matches

	// Matching thresholds
	FuzzyThreshold float64 // Threshold for fuzzy matching
	StemMinLength  int     // Minimum length for stemming

	// Result limits
	MaxResults int     // Maximum number of results to return
	MinScore   float64 // Minimum score threshold for results
}

// SearchRanking controls file type and symbol preference in search results
type SearchRanking struct {
	// Enable/disable file type ranking (default: true)
	Enabled bool

	// File type scoring preferences
	CodeFileBoost   float64 // Boost for code files like .go, .ts, .py (default: 50.0)
	DocFilePenalty  float64 // Penalty for doc files like .md, .txt (default: -20.0)
	ConfigFileBoost float64 // Boost for config files like .yaml, .json (default: 10.0)

	// Symbol preference
	RequireSymbol    bool    // If true, heavily penalize non-symbol matches (default: false)
	NonSymbolPenalty float64 // Penalty for matches without symbols (default: -30.0)

	// Extension-specific overrides (optional)
	ExtensionWeights map[string]float64
}

// Validate checks that SearchRanking values are within reasonable ranges
func (r SearchRanking) Validate() error {
	// Check for extreme values that could break search ranking
	if r.CodeFileBoost > 1000 || r.CodeFileBoost < -1000 {
		return fmt.Errorf("CodeFileBoost must be between -1000 and 1000, got %v", r.CodeFileBoost)
	}
	if r.DocFilePenalty > 0 || r.DocFilePenalty < -1000 {
		return fmt.Errorf("DocFilePenalty must be between -1000 and 0, got %v", r.DocFilePenalty)
	}
	if r.ConfigFileBoost > 1000 || r.ConfigFileBoost < -1000 {
		return fmt.Errorf("ConfigFileBoost must be between -1000 and 1000, got %v", r.ConfigFileBoost)
	}
	if r.NonSymbolPenalty > 0 || r.NonSymbolPenalty < -1000 {
		return fmt.Errorf("NonSymbolPenalty must be between -1000 and 0, got %v", r.NonSymbolPenalty)
	}

	// Check extension weights
	for ext, weight := range r.ExtensionWeights {
		if weight > 1000 || weight < -1000 {
			return fmt.Errorf("ExtensionWeights[%s] must be between -1000 and 1000, got %v", ext, weight)
		}
	}

	return nil
}

type Search struct {
	DefaultContextLines    int           // Default number of context lines to include
	MaxResults             int           // Maximum number of search results
	EnableFuzzy            bool          // Enable fuzzy matching
	MaxContextLines        int           // Maximum number of context lines
	MergeFileResults       bool          // Merge results from the same file
	EnsureCompleteStmt     bool          // Ensure complete statements in context
	IncludeLeadingComments bool          // Include leading comments in context
	Ranking                SearchRanking // File type and symbol ranking preferences
}

// FeatureFlags controls experimental features and rollback capabilities
type FeatureFlags struct {
	// Performance and reliability features
	EnableMemoryLimits         bool // Enable memory management and LRU eviction
	EnableGracefulDegradation  bool // Enable fallback to basic features on errors
	EnableRelationshipAnalysis bool // Enable universal symbol graph population (expensive)

	// Debugging and monitoring features
	EnablePerformanceMonitoring bool // Enable performance metrics collection
	EnableDetailedErrorLogging  bool // Enable detailed error context logging
	EnableFeatureFlagLogging    bool // Log feature flag state on startup
}

func Load(path string) (*Config, error) {
	return LoadWithRoot(path, "")
}

func LoadWithRoot(path string, rootDir string) (*Config, error) {
	// Determine search directory for config files
	searchDir := "."
	if rootDir != "" {
		searchDir = rootDir
	}

	// Step 1: Load global base config from ~/.lci.kdl (if exists)
	homeDir, err := os.UserHomeDir()
	var baseConfig *Config
	if err == nil {
		if globalCfg, err := LoadKDL(homeDir); err == nil && globalCfg != nil {
			baseConfig = globalCfg
		}
	}

	// Step 2: Load project-specific config from project directory
	var projectConfig *Config
	if kdlCfg, err := LoadKDL(searchDir); err == nil && kdlCfg != nil {
		projectConfig = kdlCfg
	} else if err != nil {
		return nil, err
	}

	// Step 3: Merge configs (project overrides base, but preserve base exclusions)
	if baseConfig != nil && projectConfig != nil {
		return mergeConfigs(baseConfig, projectConfig), nil
	} else if projectConfig != nil {
		return projectConfig, nil
	} else if baseConfig != nil {
		// Use base config but update project root
		baseConfig.Project.Root = searchDir
		return baseConfig, nil
	}

	// Default config
	// Use current working directory as absolute path for consistency
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "." // Fallback to relative if we can't get absolute
	}

	cfg := &Config{
		Version: 1,
		Project: Project{
			Root: cwd,
		},
		Index: Index{
			MaxFileSize:      types.DefaultMaxFileSize,
			MaxTotalSizeMB:   types.DefaultMaxTotalSizeMB,
			MaxFileCount:     types.DefaultMaxFileCount,
			FollowSymlinks:   false,
			SmartSizeControl: true,     // Enable intelligent size management
			PriorityMode:     "recent", // Prefer recently modified files
			RespectGitignore: true,     // Process .gitignore files by default
			WatchMode:        true,     // Enable file watching by default
			WatchDebounceMs:  300,      // 300ms debounce for file changes
		},
		Performance: Performance{
			MaxMemoryMB:         500,
			MaxGoroutines:       runtime.NumCPU(),
			DebounceMs:          100,
			ParallelFileWorkers: 0,    // 0 = auto-detect (NumCPU)
			IndexingTimeoutSec:  120,  // 120 seconds for large projects with -p questions
			StartupDelayMs:      1500, // 1.5 second delay to let UI become responsive
		},
		Semantic: Semantic{
			BatchSize:     100,  // Default batch size for processing
			ChannelSize:   1000, // Default channel buffer size
			MinStemLength: 3,    // Default minimum word length for stemming
			CacheSize:     1000, // Default cache size for NameSplitter
		},
		SemanticScoring: SemanticScoring{
			// Weights reflect match quality with WIDE spread for clear hierarchy
			ExactWeight:        1.0,  // Layer 1: Exact matches (query == symbol)
			SubstringWeight:    0.9,  // Layer 2: Substring containment (query IN symbol)
			AnnotationWeight:   0.85, // Layer 3: Developer intent is highly reliable
			FuzzyWeight:        0.70, // Layer 4: Typos are common, good signal
			StemmingWeight:     0.55, // Layer 5: Word forms are informative
			NameSplitWeight:    0.40, // Layer 6: camelCase/snake_case splitting
			AbbreviationWeight: 0.25, // Layer 7: Abbreviations (gui→user, udp→user)

			// Fuzzy matching threshold
			FuzzyThreshold: 0.7,

			// Minimum word length to stem
			StemMinLength: 3,

			// Result limits
			MaxResults: 10,
			MinScore:   0.2, // Lower threshold to include abbreviation matches
		},
		Search: Search{
			DefaultContextLines:    0, // Use block boundaries
			MaxResults:             100,
			EnableFuzzy:            true,
			MaxContextLines:        100,
			MergeFileResults:       true,
			EnsureCompleteStmt:     false,
			IncludeLeadingComments: true,
			Ranking: SearchRanking{
				Enabled:          true,
				CodeFileBoost:    DefaultCodeFileBoost,
				DocFilePenalty:   DefaultDocFilePenalty,
				ConfigFileBoost:  DefaultConfigFileBoost,
				RequireSymbol:    false,
				NonSymbolPenalty: DefaultNonSymbolPenalty,
			},
		},
		FeatureFlags: FeatureFlags{
			// Performance and reliability features - enable core safety features
			EnableMemoryLimits:         true,  // Enable memory management
			EnableGracefulDegradation:  true,  // Enable fallback capabilities
			EnableRelationshipAnalysis: false, // EXPENSIVE: Universal symbol graph population

			// Debugging and monitoring features - enable for better diagnostics
			EnablePerformanceMonitoring: true, // Enable performance metrics
			EnableDetailedErrorLogging:  true, // Enable detailed error logging
			EnableFeatureFlagLogging:    true, // Log feature flag state
		},
		Include: []string{},
		Exclude: []string{
			// Git metadata (never indexable)
			"**/.git/**",

			// Hidden directories (catch-all for dot directories)
			"**/.*/**", // All hidden directories

			// Package managers & dependencies
			"**/node_modules/**",
			"**/vendor/**",
			"**/bower_components/**",
			"**/jspm_packages/**",

			// Build artifacts & output
			"**/dist/**",
			"**/build/**",
			"**/out/**",
			"**/target/**", // Rust, Java
			"**/bin/**",
			"**/obj/**",    // .NET
			"**/ui/**",     // Web UI build artifacts
			"**/public/**", // Static assets
			"**/*.min.js",
			"**/*.min.css",
			"**/*.bundle.js",
			"**/*.chunk.js",
			"**/*.min.map", // Source maps for minified files

			// Test files and directories (language-agnostic patterns)
			// Go test files
			"**/*_test.go",
			"**/*_tests.go",
			// Python test files
			"**/*_test.py",
			"**/*_tests.py",
			"**/test_*.py",
			"**/tests_*.py",
			// JavaScript/TypeScript test files (Jest, Vitest, Mocha)
			"**/*.test.js",
			"**/*.test.ts",
			"**/*.test.tsx",
			"**/*.test.jsx",
			"**/*.spec.js",
			"**/*.spec.ts",
			"**/*.spec.tsx",
			"**/*.spec.jsx",
			// Generic test file prefixes (any extension)
			"**/test_*",
			"**/tests_*",
			// Test directories
			"**/__tests__/**",
			"**/test/**",
			"**/tests/**",
			"**/testdata/**",
			"**/__testdata__/**",
			"**/fixtures/**",
			"**/.test/**",
			// Ruby test files
			"**/*_test.rb",
			"**/*_spec.rb",
			// Java test files
			"**/*Test.java",
			"**/*Tests.java",
			"**/*TestCase.java",
			// C# test files
			"**/*Test.cs",
			"**/*Tests.cs",
			"**/*Test.csproj",
			// Rust test files
			"**/tests/**",
			// PHP test files
			"**/*Test.php",
			"**/*TestCase.php",
			// Kotlin test files
			"**/*Test.kt",
			"**/*Tests.kt",
			"**/*TestCase.kt",
			// Swift test files
			"**/*Test.swift",
			// Objective-C test files
			"**/*Test.m",
			"**/*Test.h",

			// Binary files (commonly found in codebases)
			"**/*.avif",  // AVIF image format
			"**/*.webp",  // WebP image format
			"**/*.wasm",  // WebAssembly
			"**/*.woff",  // Web fonts
			"**/*.woff2", // Web fonts (compressed)
			"**/*.ttf",   // TrueType fonts
			"**/*.eot",   // Embedded OpenType fonts
			"**/*.otf",   // OpenType fonts

			// Video & Audio files (binary formats)
			"**/*.mp4",
			"**/*.avi",
			"**/*.mov",
			"**/*.wmv",
			"**/*.flv",
			"**/*.mkv",
			"**/*.webm",
			"**/*.m4v",
			"**/*.mpg",
			"**/*.mpeg",
			"**/*.3gp",
			"**/*.ogv",
			"**/*.mp3",
			"**/*.wav",
			"**/*.flac",
			"**/*.aac",
			"**/*.ogg",
			"**/*.wma",
			"**/*.m4a",
			"**/*.aiff",
			"**/*.ape",

			// Office documents (binary formats)
			"**/*.doc",     // Microsoft Word
			"**/*.docx",    // Microsoft Word (XML)
			"**/*.docm",    // Microsoft Word (macro-enabled)
			"**/*.xls",     // Microsoft Excel
			"**/*.xlsx",    // Microsoft Excel (XML)
			"**/*.xlsm",    // Microsoft Excel (macro-enabled)
			"**/*.xlsb",    // Microsoft Excel (binary)
			"**/*.xlt",     // Microsoft Excel template
			"**/*.xltx",    // Microsoft Excel template (XML)
			"**/*.xltm",    // Microsoft Excel template (macro-enabled)
			"**/*.xlam",    // Microsoft Excel add-in
			"**/*.ppt",     // Microsoft PowerPoint
			"**/*.pptx",    // Microsoft PowerPoint (XML)
			"**/*.pptm",    // Microsoft PowerPoint (macro-enabled)
			"**/*.pps",     // Microsoft PowerPoint show
			"**/*.ppsx",    // Microsoft PowerPoint show (XML)
			"**/*.ppsm",    // Microsoft PowerPoint show (macro-enabled)
			"**/*.pot",     // Microsoft PowerPoint template
			"**/*.potx",    // Microsoft PowerPoint template (XML)
			"**/*.potm",    // Microsoft PowerPoint template (macro-enabled)
			"**/*.odt",     // OpenDocument Text
			"**/*.ods",     // OpenDocument Spreadsheet
			"**/*.odp",     // OpenDocument Presentation
			"**/*.rtf",     // Rich Text Format
			"**/*.pages",   // Apple Pages
			"**/*.numbers", // Apple Numbers
			"**/*.key",     // Apple Keynote

			// Editor temp files (not hidden directories)
			"**/*.swp",
			"**/*.swo",
			"**/*~",

			// Python compiled files
			"**/__pycache__/**", // Python
			"**/*.pyc",

			// OS files
			"**/Thumbs.db",
			"**/desktop.ini",

			// Logs
			"**/logs/**",
			"**/*.log",
		},
	}

	// Enrich exclusions with language-specific build artifacts
	cfg.EnrichExclusionsWithBuildArtifacts()

	return cfg, nil
}

// mergeConfigs merges a base config with a project config
// Project config takes precedence, but base exclusions are preserved
func mergeConfigs(base, project *Config) *Config {
	// Start with a copy of the project config
	merged := *project

	// Merge exclusions: combine base and project exclusions
	if len(base.Exclude) > 0 {
		// Use a map to deduplicate
		excludeMap := make(map[string]bool)

		// Add base exclusions first
		for _, pattern := range base.Exclude {
			excludeMap[pattern] = true
		}

		// Add project exclusions
		for _, pattern := range project.Exclude {
			excludeMap[pattern] = true
		}

		// Convert back to slice
		merged.Exclude = make([]string, 0, len(excludeMap))
		for pattern := range excludeMap {
			merged.Exclude = append(merged.Exclude, pattern)
		}
	}

	// Merge inclusions: project overrides base completely if specified
	if len(project.Include) == 0 && len(base.Include) > 0 {
		merged.Include = base.Include
	}

	// Use project settings for everything else (already copied above)
	// This allows project to override performance settings, search settings, etc.

	return &merged
}

// EnrichExclusionsWithBuildArtifacts detects build output directories from language configs
// and adds them to the exclusion list
func (c *Config) EnrichExclusionsWithBuildArtifacts() {
	if c.Project.Root == "" {
		return // No project root set, skip detection
	}

	detector := NewBuildArtifactDetector(c.Project.Root)
	detectedPatterns := detector.DetectOutputDirectories()

	if len(detectedPatterns) > 0 {
		// Append detected patterns to exclusions
		c.Exclude = append(c.Exclude, detectedPatterns...)
		// Deduplicate
		c.Exclude = DeduplicatePatterns(c.Exclude)
	}
}
