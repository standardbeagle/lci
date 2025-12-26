package analysis

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// PerformanceAnalyzer detects performance anti-patterns in code via AST analysis.
// Phase 1 focuses on cross-language patterns detectable without type inference:
// - Sequential awaits that could be parallelized
// - Await expressions inside loops
// - Expensive function calls inside loops
// - Nested loops (O(n²) complexity risk)
type PerformanceAnalyzer struct {
	// Configuration
	expensiveCallPatterns map[string][]ExpensiveCallConfig // Language -> patterns
	loopNodeTypes         map[string]bool                  // Node types that represent loops
	awaitNodeTypes        map[string]bool                  // Node types for await expressions
	asyncFuncNodeTypes    map[string]bool                  // Node types for async functions
}

// ExpensiveCallConfig defines a known expensive operation pattern
type ExpensiveCallConfig struct {
	Pattern     *regexp.Regexp // Compiled regex for matching
	Category    string         // "regex", "io", "network", "parse", "reflection"
	Description string         // Human-readable description
	Severity    string         // "high", "medium", "low"
}

// PerformancePattern represents a detected performance anti-pattern
type PerformancePattern struct {
	Type        string          // Pattern type
	Symbol      string          // Containing function name
	SymbolID    types.SymbolID  // Symbol ID for drill-down
	FilePath    string          // File path
	Line        int             // Line number
	Severity    string          // "high", "medium", "low"
	Description string          // Human-readable description
	Language    string          // Language where detected
	Suggestion  string          // Recommended fix
	Details     *PatternDetails // Additional details
}

// PatternDetails provides additional context for specific patterns
type PatternDetails struct {
	// For sequential-awaits pattern
	TotalAwaits         int      // Total await count in function
	ParallelizableCount int      // How many could be concurrent
	AwaitLines          []int    // Lines of parallelizable awaits
	AwaitTargets        []string // Function names being awaited

	// For nested-loops pattern
	NestingDepth  int // Depth of loop nesting
	OuterLoopLine int // Line of outermost loop
	InnerLoopLine int // Line of innermost loop

	// For expensive-call-in-loop pattern
	ExpensiveCall   string // The expensive function name
	LoopLine        int    // Line of containing loop
	CallLine        int    // Line of the expensive call
	ExpenseCategory string // "regex", "io", "network", "parse"
}

// LoopInfo tracks information about a loop during analysis
type LoopInfo struct {
	NodeType  string // "for_statement", "while_statement", etc.
	StartLine int
	EndLine   int
	Depth     int // Nesting depth (1 = top-level loop)
}

// AwaitInfo tracks information about an await expression
type AwaitInfo struct {
	Line        int
	AssignedVar string   // Variable receiving the result (empty if not assigned)
	CallTarget  string   // Function/method being awaited
	UsedVars    []string // Variables referenced in the await arguments
}

// FunctionAnalysis holds analysis data for a single function
type FunctionAnalysis struct {
	Name      string
	SymbolID  types.SymbolID
	StartLine int
	EndLine   int
	IsAsync   bool
	Loops     []LoopInfo  // All loops in the function
	Awaits    []AwaitInfo // All await expressions
	Calls     []CallInfo  // All function calls
	Language  string
	FilePath  string
}

// CallInfo tracks information about a function call
type CallInfo struct {
	Target    string // Function/method name
	Line      int
	InLoop    bool // Whether this call is inside a loop
	LoopDepth int  // Depth of containing loop (0 if not in loop)
	LoopLine  int  // Line of containing loop
}

// NewPerformanceAnalyzer creates a new performance analyzer with default configurations
func NewPerformanceAnalyzer() *PerformanceAnalyzer {
	pa := &PerformanceAnalyzer{
		expensiveCallPatterns: make(map[string][]ExpensiveCallConfig),
		loopNodeTypes:         make(map[string]bool),
		awaitNodeTypes:        make(map[string]bool),
		asyncFuncNodeTypes:    make(map[string]bool),
	}

	pa.initializeLoopNodeTypes()
	pa.initializeAwaitNodeTypes()
	pa.initializeAsyncFuncNodeTypes()
	pa.initializeExpensiveCallPatterns()

	return pa
}

// initializeLoopNodeTypes sets up loop node type detection
func (pa *PerformanceAnalyzer) initializeLoopNodeTypes() {
	loopTypes := []string{
		// Go
		"for_statement",
		"for_range_statement",
		// JavaScript/TypeScript
		"for_in_statement",
		"for_of_statement",
		// Python
		"for_statement",
		"while_statement",
		// Common across languages
		"while_statement",
		"do_while_statement",
		"do_statement",
		// Rust
		"loop_expression",
		"while_expression",
		"for_expression",
		// Java/C#
		"for_each_statement",
		"enhanced_for_statement",
	}
	for _, t := range loopTypes {
		pa.loopNodeTypes[t] = true
	}
}

// initializeAwaitNodeTypes sets up await expression detection
func (pa *PerformanceAnalyzer) initializeAwaitNodeTypes() {
	awaitTypes := []string{
		"await_expression", // JS/TS, C#, Rust
		"await",            // Python
	}
	for _, t := range awaitTypes {
		pa.awaitNodeTypes[t] = true
	}
}

// initializeAsyncFuncNodeTypes sets up async function detection
func (pa *PerformanceAnalyzer) initializeAsyncFuncNodeTypes() {
	// These are markers that indicate a function is async
	// The actual detection needs to check for "async" keyword in function declaration
	asyncTypes := []string{
		"async_function_declaration",
		"async_function_expression",
		"async_arrow_function",
		"async_method_definition",
	}
	for _, t := range asyncTypes {
		pa.asyncFuncNodeTypes[t] = true
	}
}

// initializeExpensiveCallPatterns sets up known expensive operation patterns per language
func (pa *PerformanceAnalyzer) initializeExpensiveCallPatterns() {
	// Go expensive patterns
	pa.expensiveCallPatterns["go"] = []ExpensiveCallConfig{
		{mustCompileRegex(`regexp\.(Compile|MustCompile|Match|MatchString)`), "regex", "Regex compilation/matching", "high"},
		{mustCompileRegex(`json\.(Unmarshal|Marshal|NewDecoder|NewEncoder)`), "parse", "JSON parsing", "medium"},
		{mustCompileRegex(`xml\.(Unmarshal|Marshal)`), "parse", "XML parsing", "medium"},
		{mustCompileRegex(`os\.(Open|Create|ReadFile|WriteFile|Stat)`), "io", "File I/O operation", "high"},
		{mustCompileRegex(`ioutil\.(ReadFile|WriteFile|ReadAll)`), "io", "File I/O operation", "high"},
		{mustCompileRegex(`http\.(Get|Post|Do)`), "network", "HTTP request", "high"},
		{mustCompileRegex(`sql\.DB\.(Query|QueryRow|Exec)`), "io", "Database query", "high"},
		{mustCompileRegex(`reflect\.(ValueOf|TypeOf)`), "reflection", "Reflection operation", "medium"},
	}

	// JavaScript/TypeScript expensive patterns
	pa.expensiveCallPatterns["javascript"] = []ExpensiveCallConfig{
		{mustCompileRegex(`new\s+RegExp`), "regex", "Regex construction", "high"},
		{mustCompileRegex(`JSON\.(parse|stringify)`), "parse", "JSON parsing", "medium"},
		{mustCompileRegex(`fetch\s*\(`), "network", "Network fetch", "high"},
		{mustCompileRegex(`axios\.(get|post|put|delete|request)`), "network", "HTTP request", "high"},
		{mustCompileRegex(`fs\.(readFile|writeFile|readdir|stat)`), "io", "File I/O operation", "high"},
		{mustCompileRegex(`localStorage\.(getItem|setItem)`), "io", "Storage operation", "medium"},
		{mustCompileRegex(`document\.(querySelector|querySelectorAll|getElementById)`), "dom", "DOM query", "medium"},
	}
	pa.expensiveCallPatterns["typescript"] = pa.expensiveCallPatterns["javascript"]

	// Python expensive patterns
	pa.expensiveCallPatterns["python"] = []ExpensiveCallConfig{
		{mustCompileRegex(`re\.(compile|match|search|findall)`), "regex", "Regex operation", "high"},
		{mustCompileRegex(`json\.(loads|dumps|load|dump)`), "parse", "JSON parsing", "medium"},
		{mustCompileRegex(`open\s*\(`), "io", "File I/O operation", "high"},
		{mustCompileRegex(`requests\.(get|post|put|delete)`), "network", "HTTP request", "high"},
		{mustCompileRegex(`urllib\.request\.urlopen`), "network", "URL fetch", "high"},
		{mustCompileRegex(`pickle\.(load|dump|loads|dumps)`), "parse", "Pickle serialization", "medium"},
		{mustCompileRegex(`yaml\.(load|dump|safe_load)`), "parse", "YAML parsing", "medium"},
	}

	// Rust expensive patterns
	pa.expensiveCallPatterns["rust"] = []ExpensiveCallConfig{
		{mustCompileRegex(`Regex::new`), "regex", "Regex compilation", "high"},
		{mustCompileRegex(`serde_json::(from_str|to_string|from_reader)`), "parse", "JSON parsing", "medium"},
		{mustCompileRegex(`File::(open|create)`), "io", "File I/O operation", "high"},
		{mustCompileRegex(`reqwest::(get|Client)`), "network", "HTTP request", "high"},
		{mustCompileRegex(`tokio::fs::`), "io", "Async file I/O", "high"},
	}

	// Java expensive patterns
	pa.expensiveCallPatterns["java"] = []ExpensiveCallConfig{
		{mustCompileRegex(`Pattern\.(compile|matches)`), "regex", "Regex compilation", "high"},
		{mustCompileRegex(`new\s+ObjectMapper`), "parse", "JSON mapper creation", "high"},
		{mustCompileRegex(`Files\.(readAllLines|readAllBytes|write)`), "io", "File I/O operation", "high"},
		{mustCompileRegex(`new\s+URL\(`), "network", "URL construction", "medium"},
		{mustCompileRegex(`HttpClient\.`), "network", "HTTP operation", "high"},
		{mustCompileRegex(`Class\.forName`), "reflection", "Reflection operation", "medium"},
	}

	// C# expensive patterns
	pa.expensiveCallPatterns["csharp"] = []ExpensiveCallConfig{
		{mustCompileRegex(`new\s+Regex\(`), "regex", "Regex construction", "high"},
		{mustCompileRegex(`Regex\.(Match|Matches|IsMatch)`), "regex", "Regex operation", "high"},
		{mustCompileRegex(`JsonSerializer\.(Serialize|Deserialize)`), "parse", "JSON serialization", "medium"},
		{mustCompileRegex(`File\.(ReadAllText|WriteAllText|ReadAllLines)`), "io", "File I/O operation", "high"},
		{mustCompileRegex(`HttpClient\.(GetAsync|PostAsync|SendAsync)`), "network", "HTTP request", "high"},
	}
}

func mustCompileRegex(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

// IsLoopNode checks if a node type represents a loop construct
func (pa *PerformanceAnalyzer) IsLoopNode(nodeType string) bool {
	return pa.loopNodeTypes[nodeType]
}

// IsAwaitNode checks if a node type represents an await expression
func (pa *PerformanceAnalyzer) IsAwaitNode(nodeType string) bool {
	return pa.awaitNodeTypes[nodeType]
}

// AnalyzeFunction analyzes a single function for performance anti-patterns
func (pa *PerformanceAnalyzer) AnalyzeFunction(analysis *FunctionAnalysis) []PerformancePattern {
	var patterns []PerformancePattern

	// 1. Check for sequential awaits (only in async functions)
	if analysis.IsAsync && len(analysis.Awaits) >= 2 {
		if pattern := pa.detectSequentialAwaits(analysis); pattern != nil {
			patterns = append(patterns, *pattern)
		}
	}

	// 2. Check for await in loop
	for _, await := range analysis.Awaits {
		if pattern := pa.detectAwaitInLoop(analysis, await); pattern != nil {
			patterns = append(patterns, *pattern)
		}
	}

	// 3. Check for expensive calls in loops
	for _, call := range analysis.Calls {
		if call.InLoop {
			if pattern := pa.detectExpensiveCallInLoop(analysis, call); pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	// 4. Check for nested loops
	if pattern := pa.detectNestedLoops(analysis); pattern != nil {
		patterns = append(patterns, *pattern)
	}

	return patterns
}

// detectSequentialAwaits checks for awaits that could be parallelized
func (pa *PerformanceAnalyzer) detectSequentialAwaits(analysis *FunctionAnalysis) *PerformancePattern {
	if len(analysis.Awaits) < 2 {
		return nil
	}

	// Build dependency graph: await[i] depends on await[j] if await[i].UsedVars contains await[j].AssignedVar
	awaitCount := len(analysis.Awaits)
	dependencies := make([][]bool, awaitCount)
	for i := range dependencies {
		dependencies[i] = make([]bool, awaitCount)
	}

	// Check dependencies
	for i, laterAwait := range analysis.Awaits {
		for j, earlierAwait := range analysis.Awaits {
			if j >= i {
				continue // Only check earlier awaits
			}
			if earlierAwait.AssignedVar != "" {
				for _, usedVar := range laterAwait.UsedVars {
					if usedVar == earlierAwait.AssignedVar {
						dependencies[i][j] = true
						break
					}
				}
			}
		}
	}

	// Find independent awaits (no dependencies between them)
	// An await is independent if it doesn't depend on any other await
	var independentAwaits []int
	for i := range analysis.Awaits {
		hasNoDeps := true
		for j := 0; j < i; j++ {
			if dependencies[i][j] {
				hasNoDeps = false
				break
			}
		}
		if hasNoDeps {
			independentAwaits = append(independentAwaits, i)
		}
	}

	// If we have 2+ independent awaits, they could be parallelized
	if len(independentAwaits) >= 2 {
		var awaitLines []int
		var awaitTargets []string
		for _, idx := range independentAwaits {
			awaitLines = append(awaitLines, analysis.Awaits[idx].Line)
			awaitTargets = append(awaitTargets, analysis.Awaits[idx].CallTarget)
		}

		return &PerformancePattern{
			Type:        "sequential-awaits",
			Symbol:      analysis.Name,
			SymbolID:    analysis.SymbolID,
			FilePath:    analysis.FilePath,
			Line:        analysis.StartLine,
			Severity:    pa.calculateSequentialAwaitsSeverity(len(independentAwaits)),
			Description: fmt.Sprintf("%d of %d awaits could be parallelized", len(independentAwaits), awaitCount),
			Language:    analysis.Language,
			Suggestion:  pa.getParallelizationSuggestion(analysis.Language),
			Details: &PatternDetails{
				TotalAwaits:         awaitCount,
				ParallelizableCount: len(independentAwaits),
				AwaitLines:          awaitLines,
				AwaitTargets:        awaitTargets,
			},
		}
	}

	return nil
}

func (pa *PerformanceAnalyzer) calculateSequentialAwaitsSeverity(parallelizableCount int) string {
	if parallelizableCount >= 4 {
		return "high"
	}
	if parallelizableCount >= 3 {
		return "medium"
	}
	return "low"
}

func (pa *PerformanceAnalyzer) getParallelizationSuggestion(language string) string {
	switch language {
	case "javascript", "typescript":
		return "Use Promise.all([...]) to run awaits concurrently"
	case "python":
		return "Use asyncio.gather(...) to run awaits concurrently"
	case "csharp":
		return "Use Task.WhenAll(...) to run awaits concurrently"
	case "rust":
		return "Use tokio::join!() or futures::join!() to run awaits concurrently"
	default:
		return "Consider parallelizing independent async operations"
	}
}

// detectAwaitInLoop checks if an await is inside a loop
func (pa *PerformanceAnalyzer) detectAwaitInLoop(analysis *FunctionAnalysis, await AwaitInfo) *PerformancePattern {
	// Check if await line is within any loop's range
	for _, loop := range analysis.Loops {
		if await.Line >= loop.StartLine && await.Line <= loop.EndLine {
			return &PerformancePattern{
				Type:        "await-in-loop",
				Symbol:      analysis.Name,
				SymbolID:    analysis.SymbolID,
				FilePath:    analysis.FilePath,
				Line:        await.Line,
				Severity:    "high",
				Description: fmt.Sprintf("await '%s' inside loop causes sequential execution", await.CallTarget),
				Language:    analysis.Language,
				Suggestion:  pa.getAwaitInLoopSuggestion(analysis.Language),
				Details: &PatternDetails{
					LoopLine:        loop.StartLine,
					ExpenseCategory: "async",
				},
			}
		}
	}
	return nil
}

func (pa *PerformanceAnalyzer) getAwaitInLoopSuggestion(language string) string {
	switch language {
	case "javascript", "typescript":
		return "Collect promises and use Promise.all() outside the loop"
	case "python":
		return "Collect coroutines and use asyncio.gather() outside the loop"
	case "csharp":
		return "Collect tasks and use Task.WhenAll() outside the loop"
	case "rust":
		return "Use futures::stream or collect futures for batch execution"
	default:
		return "Batch async operations instead of awaiting in loop"
	}
}

// detectExpensiveCallInLoop checks for expensive operations inside loops
func (pa *PerformanceAnalyzer) detectExpensiveCallInLoop(analysis *FunctionAnalysis, call CallInfo) *PerformancePattern {
	patterns, exists := pa.expensiveCallPatterns[analysis.Language]
	if !exists {
		return nil
	}

	for _, expPattern := range patterns {
		if expPattern.Pattern.MatchString(call.Target) {
			return &PerformancePattern{
				Type:        "expensive-call-in-loop",
				Symbol:      analysis.Name,
				SymbolID:    analysis.SymbolID,
				FilePath:    analysis.FilePath,
				Line:        call.Line,
				Severity:    expPattern.Severity,
				Description: fmt.Sprintf("%s (%s) called inside loop", call.Target, expPattern.Description),
				Language:    analysis.Language,
				Suggestion:  pa.getExpensiveCallSuggestion(expPattern.Category, analysis.Language),
				Details: &PatternDetails{
					ExpensiveCall:   call.Target,
					LoopLine:        call.LoopLine,
					CallLine:        call.Line,
					ExpenseCategory: expPattern.Category,
				},
			}
		}
	}
	return nil
}

func (pa *PerformanceAnalyzer) getExpensiveCallSuggestion(category, language string) string {
	switch category {
	case "regex":
		switch language {
		case "go":
			return "Compile regex once outside the loop using regexp.MustCompile"
		case "javascript", "typescript":
			return "Create RegExp object once outside the loop"
		case "python":
			return "Compile regex once with re.compile() outside the loop"
		case "java":
			return "Compile Pattern once outside the loop"
		case "rust":
			return "Use lazy_static! or once_cell to compile regex once"
		default:
			return "Pre-compile regex outside the loop"
		}
	case "io":
		return "Consider batching I/O operations or caching results"
	case "network":
		return "Batch network requests or use connection pooling"
	case "parse":
		return "Cache parsed objects if data doesn't change, or batch parsing"
	case "reflection":
		return "Cache reflection results or use generated code"
	case "dom":
		return "Cache DOM references outside the loop"
	default:
		return "Move expensive operation outside the loop if possible"
	}
}

// detectNestedLoops checks for nested loop structures
func (pa *PerformanceAnalyzer) detectNestedLoops(analysis *FunctionAnalysis) *PerformancePattern {
	if len(analysis.Loops) < 2 {
		return nil
	}

	// Find the deepest nesting
	maxDepth := 0
	var outerLoop, innerLoop *LoopInfo
	for i := range analysis.Loops {
		if analysis.Loops[i].Depth > maxDepth {
			maxDepth = analysis.Loops[i].Depth
			innerLoop = &analysis.Loops[i]
		}
		if analysis.Loops[i].Depth == 1 {
			outerLoop = &analysis.Loops[i]
		}
	}

	// Only report if nesting depth >= 2
	if maxDepth >= 2 && outerLoop != nil && innerLoop != nil {
		severity := "low"
		if maxDepth >= 3 {
			severity = "high"
		} else if maxDepth >= 2 {
			severity = "medium"
		}

		return &PerformancePattern{
			Type:        "nested-loops",
			Symbol:      analysis.Name,
			SymbolID:    analysis.SymbolID,
			FilePath:    analysis.FilePath,
			Line:        outerLoop.StartLine,
			Severity:    severity,
			Description: fmt.Sprintf("Nested loops with depth %d (potential O(n^%d) complexity)", maxDepth, maxDepth),
			Language:    analysis.Language,
			Suggestion:  "Consider using a Map/Set for O(1) lookups, or restructure algorithm",
			Details: &PatternDetails{
				NestingDepth:  maxDepth,
				OuterLoopLine: outerLoop.StartLine,
				InnerLoopLine: innerLoop.StartLine,
			},
		}
	}

	return nil
}

// AnalyzeFile analyzes all functions in a file for performance anti-patterns
func (pa *PerformanceAnalyzer) AnalyzeFile(fileInfo *types.FileInfo, functions []*FunctionAnalysis) []PerformancePattern {
	var allPatterns []PerformancePattern

	for _, fn := range functions {
		patterns := pa.AnalyzeFunction(fn)
		allPatterns = append(allPatterns, patterns...)
	}

	return allPatterns
}

// GetLanguageFromExt returns the language name from file extension
func GetLanguageFromExt(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return "unknown"
	}
}

// GetLanguageFromPath extracts language from file path
func GetLanguageFromPath(path string) string {
	ext := filepath.Ext(path)
	return GetLanguageFromExt(ext)
}
