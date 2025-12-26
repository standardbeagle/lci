// Package git provides git integration for analyzing code changes.
// It supports analyzing staged changes, work-in-progress, specific commits,
// and commit ranges for duplicate detection and naming consistency analysis.
package git

// AnalysisScope defines what changes to analyze
type AnalysisScope string

const (
	// ScopeStaged analyzes only staged changes (git diff --cached)
	ScopeStaged AnalysisScope = "staged"
	// ScopeWIP analyzes all uncommitted changes (staged + unstaged)
	ScopeWIP AnalysisScope = "wip"
	// ScopeCommit analyzes a specific commit vs its parent
	ScopeCommit AnalysisScope = "commit"
	// ScopeRange analyzes a commit range (base..target)
	ScopeRange AnalysisScope = "range"
)

// AnalysisParams configures the git change analysis
type AnalysisParams struct {
	// Scope determines what changes to analyze
	Scope AnalysisScope `json:"scope"`

	// BaseRef is the reference for the base index (default depends on scope)
	// For staged/wip: HEAD
	// For commit: parent commit
	// For range: specified base
	BaseRef string `json:"base_ref,omitempty"`

	// TargetRef is the reference for the working index
	// For range mode, this is the target commit
	TargetRef string `json:"target_ref,omitempty"`

	// Focus specifies which analyses to perform
	// Options: "duplicates", "naming"
	// Default: both
	Focus []string `json:"focus,omitempty"`

	// SimilarityThreshold for duplicate/naming detection (0.0-1.0)
	// Default: 0.8
	SimilarityThreshold float64 `json:"similarity_threshold,omitempty"`

	// MaxFindings limits findings per category
	// Default: 20
	MaxFindings int `json:"max_findings,omitempty"`
}

// DefaultAnalysisParams returns default analysis parameters
func DefaultAnalysisParams() AnalysisParams {
	return AnalysisParams{
		Scope:               ScopeStaged,
		Focus:               []string{"duplicates", "naming"},
		SimilarityThreshold: 0.8,
		MaxFindings:         20,
	}
}

// HasFocus checks if a specific focus area is enabled
func (p *AnalysisParams) HasFocus(focus string) bool {
	if len(p.Focus) == 0 {
		// Default: all focus areas enabled
		return true
	}
	for _, f := range p.Focus {
		if f == focus || f == "all" {
			return true
		}
	}
	return false
}

// FileChangeStatus indicates the type of change to a file
type FileChangeStatus string

const (
	FileStatusAdded    FileChangeStatus = "added"
	FileStatusModified FileChangeStatus = "modified"
	FileStatusDeleted  FileChangeStatus = "deleted"
	FileStatusRenamed  FileChangeStatus = "renamed"
	FileStatusCopied   FileChangeStatus = "copied"
)

// ChangedFile represents a file affected by git changes
type ChangedFile struct {
	// Path is the current file path
	Path string `json:"path"`

	// OldPath is the previous path (for renames)
	OldPath string `json:"old_path,omitempty"`

	// Status indicates the type of change
	Status FileChangeStatus `json:"status"`

	// LinesAdded is the number of lines added
	LinesAdded int `json:"lines_added"`

	// LinesDeleted is the number of lines deleted
	LinesDeleted int `json:"lines_deleted"`
}

// DiffStats provides summary statistics for a diff
type DiffStats struct {
	FilesAdded    int `json:"files_added"`
	FilesModified int `json:"files_modified"`
	FilesDeleted  int `json:"files_deleted"`
	FilesRenamed  int `json:"files_renamed"`
	TotalAdded    int `json:"total_lines_added"`
	TotalDeleted  int `json:"total_lines_deleted"`
}

// DiffSize categorizes the size of a diff for report pagination
type DiffSize string

const (
	DiffSizeSmall  DiffSize = "small"  // < 10 files
	DiffSizeMedium DiffSize = "medium" // 10-50 files
	DiffSizeLarge  DiffSize = "large"  // > 50 files
)

// CategorizeDiffSize determines the diff size category
func CategorizeDiffSize(files []ChangedFile) DiffSize {
	count := len(files)
	switch {
	case count < 10:
		return DiffSizeSmall
	case count <= 50:
		return DiffSizeMedium
	default:
		return DiffSizeLarge
	}
}

// SymbolDiff represents the difference in symbols between two indexes
type SymbolDiff struct {
	Added    []SymbolInfo `json:"added"`
	Removed  []SymbolInfo `json:"removed"`
	Modified []SymbolInfo `json:"modified"`
}

// SymbolInfo provides details about a symbol for analysis
type SymbolInfo struct {
	// Name is the symbol name
	Name string `json:"name"`

	// Type is the symbol type (function, class, etc.) as a string
	Type string `json:"type"`

	// FilePath is the file containing the symbol
	FilePath string `json:"file_path"`

	// Line is the line number
	Line int `json:"line"`

	// EndLine is the ending line number
	EndLine int `json:"end_line,omitempty"`

	// Complexity is the cyclomatic complexity (if available)
	Complexity int `json:"complexity,omitempty"`

	// Content is the symbol's code content (for duplicate detection)
	Content string `json:"-"`
}

// CodeLocation identifies a specific location in code
type CodeLocation struct {
	// FilePath is the file path
	FilePath string `json:"file_path"`

	// StartLine is the starting line number
	StartLine int `json:"start_line"`

	// EndLine is the ending line number
	EndLine int `json:"end_line"`

	// SymbolName is the name of the containing symbol (if applicable)
	SymbolName string `json:"symbol_name,omitempty"`

	// Snippet is a code snippet for context
	Snippet string `json:"snippet,omitempty"`
}

// NamingIssueType categorizes naming consistency issues
type NamingIssueType string

const (
	// NamingIssueCaseMismatch indicates camelCase vs PascalCase vs snake_case mismatch
	NamingIssueCaseMismatch NamingIssueType = "case_mismatch"

	// NamingIssueSimilarExists indicates similar names already exist in codebase
	NamingIssueSimilarExists NamingIssueType = "similar_exists"

	// NamingIssueAbbreviation indicates abbreviation inconsistency (getUsr vs getUser)
	NamingIssueAbbreviation NamingIssueType = "abbreviation"
)

// CaseStyle represents a naming convention style
type CaseStyle string

const (
	CaseStyleCamelCase  CaseStyle = "camelCase"
	CaseStylePascalCase CaseStyle = "PascalCase"
	CaseStyleSnakeCase  CaseStyle = "snake_case"
	CaseStyleKebabCase  CaseStyle = "kebab-case"
	CaseStyleUnknown    CaseStyle = "unknown"
)

// Language represents a programming language for naming conventions
type Language string

const (
	LangGo         Language = "go"
	LangJavaScript Language = "javascript"
	LangTypeScript Language = "typescript"
	LangPython     Language = "python"
	LangRust       Language = "rust"
	LangJava       Language = "java"
	LangCSharp     Language = "csharp"
	LangCpp        Language = "cpp"
	LangC          Language = "c"
	LangPHP        Language = "php"
	LangRuby       Language = "ruby"
	LangSwift      Language = "swift"
	LangKotlin     Language = "kotlin"
	LangScala      Language = "scala"
	LangZig        Language = "zig"
	LangUnknown    Language = "unknown"
)

// SymbolKind categorizes symbols for naming convention purposes
type SymbolKind string

const (
	KindFunction    SymbolKind = "function"
	KindMethod      SymbolKind = "method"
	KindClass       SymbolKind = "class"
	KindInterface   SymbolKind = "interface"
	KindStruct      SymbolKind = "struct"
	KindType        SymbolKind = "type"
	KindConstant    SymbolKind = "constant"
	KindVariable    SymbolKind = "variable"
	KindField       SymbolKind = "field"
	KindEnum        SymbolKind = "enum"
	KindEnumMember  SymbolKind = "enum_member"
	KindModule      SymbolKind = "module"
	KindNamespace   SymbolKind = "namespace"
	KindProperty    SymbolKind = "property"
	KindUnknownKind SymbolKind = "unknown"
)

// NamingConvention defines expected case styles for a language
type NamingConvention struct {
	// ExpectedStyles maps symbol kinds to their expected case styles
	// Multiple styles means any of them is acceptable
	ExpectedStyles map[SymbolKind][]CaseStyle
	// Description provides human-readable explanation
	Description string
}

// LanguageNamingConventions maps languages to their naming conventions
var LanguageNamingConventions = map[Language]NamingConvention{
	LangGo: {
		Description: "Go uses PascalCase for exported, camelCase for unexported",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:   {CaseStylePascalCase, CaseStyleCamelCase}, // Both valid (exported vs unexported)
			KindMethod:     {CaseStylePascalCase, CaseStyleCamelCase},
			KindClass:      {CaseStylePascalCase, CaseStyleCamelCase}, // Go doesn't have classes, but map to struct
			KindInterface:  {CaseStylePascalCase, CaseStyleCamelCase}, // Interfaces can be unexported too
			KindStruct:     {CaseStylePascalCase, CaseStyleCamelCase}, // Structs can be unexported
			KindType:       {CaseStylePascalCase, CaseStyleCamelCase},
			KindConstant:   {CaseStylePascalCase, CaseStyleCamelCase}, // Go doesn't use SCREAMING_SNAKE
			KindVariable:   {CaseStylePascalCase, CaseStyleCamelCase},
			KindField:      {CaseStylePascalCase, CaseStyleCamelCase},
			KindEnum:       {CaseStylePascalCase, CaseStyleCamelCase}, // Go enums can be unexported
			KindEnumMember: {CaseStylePascalCase, CaseStyleCamelCase},
		},
	},
	LangJavaScript: {
		Description: "JavaScript uses camelCase for functions/variables, PascalCase for classes",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:  {CaseStyleCamelCase},
			KindMethod:    {CaseStyleCamelCase},
			KindClass:     {CaseStylePascalCase},
			KindInterface: {CaseStylePascalCase},
			KindConstant:  {CaseStyleCamelCase, CaseStyleSnakeCase}, // SCREAMING_SNAKE or camelCase
			KindVariable:  {CaseStyleCamelCase},
			KindField:     {CaseStyleCamelCase},
			KindProperty:  {CaseStyleCamelCase},
		},
	},
	LangTypeScript: {
		Description: "TypeScript uses camelCase for functions/variables, PascalCase for types/classes",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:   {CaseStyleCamelCase},
			KindMethod:     {CaseStyleCamelCase},
			KindClass:      {CaseStylePascalCase},
			KindInterface:  {CaseStylePascalCase},
			KindType:       {CaseStylePascalCase},
			KindConstant:   {CaseStyleCamelCase, CaseStyleSnakeCase},
			KindVariable:   {CaseStyleCamelCase},
			KindField:      {CaseStyleCamelCase},
			KindProperty:   {CaseStyleCamelCase},
			KindEnum:       {CaseStylePascalCase},
			KindEnumMember: {CaseStylePascalCase, CaseStyleSnakeCase},
		},
	},
	LangPython: {
		Description: "Python uses snake_case for functions/variables, PascalCase for classes",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction: {CaseStyleSnakeCase},
			KindMethod:   {CaseStyleSnakeCase},
			KindClass:    {CaseStylePascalCase},
			KindConstant: {CaseStyleSnakeCase}, // SCREAMING_SNAKE_CASE (detected as snake_case)
			KindVariable: {CaseStyleSnakeCase},
			KindField:    {CaseStyleSnakeCase},
			KindProperty: {CaseStyleSnakeCase},
			KindModule:   {CaseStyleSnakeCase},
		},
	},
	LangRust: {
		Description: "Rust uses snake_case for functions/variables, PascalCase for types",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:   {CaseStyleSnakeCase},
			KindMethod:     {CaseStyleSnakeCase},
			KindClass:      {CaseStylePascalCase}, // No classes, but structs
			KindInterface:  {CaseStylePascalCase}, // Traits
			KindStruct:     {CaseStylePascalCase},
			KindType:       {CaseStylePascalCase},
			KindConstant:   {CaseStyleSnakeCase}, // SCREAMING_SNAKE_CASE
			KindVariable:   {CaseStyleSnakeCase},
			KindField:      {CaseStyleSnakeCase},
			KindEnum:       {CaseStylePascalCase},
			KindEnumMember: {CaseStylePascalCase},
			KindModule:     {CaseStyleSnakeCase},
		},
	},
	LangJava: {
		Description: "Java uses camelCase for methods/variables, PascalCase for classes",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:   {CaseStyleCamelCase}, // Static methods
			KindMethod:     {CaseStyleCamelCase},
			KindClass:      {CaseStylePascalCase},
			KindInterface:  {CaseStylePascalCase},
			KindConstant:   {CaseStyleSnakeCase}, // SCREAMING_SNAKE_CASE
			KindVariable:   {CaseStyleCamelCase},
			KindField:      {CaseStyleCamelCase},
			KindEnum:       {CaseStylePascalCase},
			KindEnumMember: {CaseStyleSnakeCase}, // SCREAMING_SNAKE_CASE
		},
	},
	LangCSharp: {
		Description: "C# uses PascalCase for public members, camelCase for private",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:   {CaseStylePascalCase},
			KindMethod:     {CaseStylePascalCase, CaseStyleCamelCase}, // Private can be camelCase
			KindClass:      {CaseStylePascalCase},
			KindInterface:  {CaseStylePascalCase}, // Usually starts with I
			KindStruct:     {CaseStylePascalCase},
			KindConstant:   {CaseStylePascalCase},
			KindVariable:   {CaseStyleCamelCase},
			KindField:      {CaseStyleCamelCase, CaseStylePascalCase}, // _camelCase for private
			KindProperty:   {CaseStylePascalCase},
			KindEnum:       {CaseStylePascalCase},
			KindEnumMember: {CaseStylePascalCase},
			KindNamespace:  {CaseStylePascalCase},
		},
	},
	LangCpp: {
		Description: "C++ conventions vary, commonly snake_case or camelCase for functions",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:  {CaseStyleSnakeCase, CaseStyleCamelCase, CaseStylePascalCase},
			KindMethod:    {CaseStyleSnakeCase, CaseStyleCamelCase, CaseStylePascalCase},
			KindClass:     {CaseStylePascalCase},
			KindStruct:    {CaseStylePascalCase, CaseStyleSnakeCase},
			KindConstant:  {CaseStyleSnakeCase}, // SCREAMING_SNAKE_CASE
			KindVariable:  {CaseStyleSnakeCase, CaseStyleCamelCase},
			KindField:     {CaseStyleSnakeCase, CaseStyleCamelCase},
			KindNamespace: {CaseStyleSnakeCase, CaseStylePascalCase},
		},
	},
	LangC: {
		Description: "C uses snake_case for functions/variables",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction: {CaseStyleSnakeCase},
			KindStruct:   {CaseStyleSnakeCase, CaseStylePascalCase},
			KindConstant: {CaseStyleSnakeCase}, // SCREAMING_SNAKE_CASE
			KindVariable: {CaseStyleSnakeCase},
			KindField:    {CaseStyleSnakeCase},
		},
	},
	LangRuby: {
		Description: "Ruby uses snake_case for methods/variables, PascalCase for classes",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction: {CaseStyleSnakeCase},
			KindMethod:   {CaseStyleSnakeCase},
			KindClass:    {CaseStylePascalCase},
			KindModule:   {CaseStylePascalCase},
			KindConstant: {CaseStyleSnakeCase}, // SCREAMING_SNAKE_CASE
			KindVariable: {CaseStyleSnakeCase},
			KindField:    {CaseStyleSnakeCase},
		},
	},
	LangSwift: {
		Description: "Swift uses camelCase for functions/properties, PascalCase for types",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:   {CaseStyleCamelCase},
			KindMethod:     {CaseStyleCamelCase},
			KindClass:      {CaseStylePascalCase},
			KindInterface:  {CaseStylePascalCase}, // Protocols
			KindStruct:     {CaseStylePascalCase},
			KindType:       {CaseStylePascalCase},
			KindConstant:   {CaseStyleCamelCase},
			KindVariable:   {CaseStyleCamelCase},
			KindProperty:   {CaseStyleCamelCase},
			KindEnum:       {CaseStylePascalCase},
			KindEnumMember: {CaseStyleCamelCase},
		},
	},
	LangKotlin: {
		Description: "Kotlin uses camelCase for functions/properties, PascalCase for classes",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:   {CaseStyleCamelCase},
			KindMethod:     {CaseStyleCamelCase},
			KindClass:      {CaseStylePascalCase},
			KindInterface:  {CaseStylePascalCase},
			KindConstant:   {CaseStyleSnakeCase}, // SCREAMING_SNAKE_CASE
			KindVariable:   {CaseStyleCamelCase},
			KindProperty:   {CaseStyleCamelCase},
			KindEnum:       {CaseStylePascalCase},
			KindEnumMember: {CaseStyleSnakeCase},
		},
	},
	LangScala: {
		Description: "Scala uses camelCase for methods/values, PascalCase for types",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction:  {CaseStyleCamelCase},
			KindMethod:    {CaseStyleCamelCase},
			KindClass:     {CaseStylePascalCase},
			KindInterface: {CaseStylePascalCase}, // Traits
			KindType:      {CaseStylePascalCase},
			KindConstant:  {CaseStylePascalCase}, // Scala prefers PascalCase for vals
			KindVariable:  {CaseStyleCamelCase},
		},
	},
	LangZig: {
		Description: "Zig uses camelCase for functions, PascalCase for types",
		ExpectedStyles: map[SymbolKind][]CaseStyle{
			KindFunction: {CaseStyleCamelCase},
			KindStruct:   {CaseStylePascalCase},
			KindType:     {CaseStylePascalCase},
			KindConstant: {CaseStyleSnakeCase},
			KindVariable: {CaseStyleSnakeCase, CaseStyleCamelCase},
		},
	},
}

// GetLanguageFromPath returns the language based on file extension
func GetLanguageFromPath(path string) Language {
	ext := ""
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			ext = path[i:]
			break
		}
	}

	switch ext {
	case ".go":
		return LangGo
	case ".js", ".jsx", ".mjs", ".cjs":
		return LangJavaScript
	case ".ts", ".tsx", ".mts", ".cts":
		return LangTypeScript
	case ".py", ".pyw", ".pyi":
		return LangPython
	case ".rs":
		return LangRust
	case ".java":
		return LangJava
	case ".cs":
		return LangCSharp
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx", ".h++":
		return LangCpp
	case ".c", ".h":
		return LangC
	case ".php":
		return LangPHP
	case ".rb":
		return LangRuby
	case ".swift":
		return LangSwift
	case ".kt", ".kts":
		return LangKotlin
	case ".scala", ".sc":
		return LangScala
	case ".zig":
		return LangZig
	default:
		return LangUnknown
	}
}

// SymbolTypeToKind converts a symbol type string to SymbolKind
func SymbolTypeToKind(symbolType string) SymbolKind {
	switch symbolType {
	case "function":
		return KindFunction
	case "method":
		return KindMethod
	case "class":
		return KindClass
	case "interface":
		return KindInterface
	case "struct":
		return KindStruct
	case "type", "type_alias":
		return KindType
	case "constant":
		return KindConstant
	case "variable":
		return KindVariable
	case "field":
		return KindField
	case "enum":
		return KindEnum
	case "enum_member":
		return KindEnumMember
	case "module":
		return KindModule
	case "namespace":
		return KindNamespace
	case "property":
		return KindProperty
	default:
		return KindUnknownKind
	}
}

// IsValidCaseStyle checks if a case style is valid for a symbol in a language
func IsValidCaseStyle(lang Language, kind SymbolKind, style CaseStyle) bool {
	conv, ok := LanguageNamingConventions[lang]
	if !ok {
		// Unknown language - accept anything
		return true
	}

	expectedStyles, ok := conv.ExpectedStyles[kind]
	if !ok {
		// No rules for this symbol kind - accept anything
		return true
	}

	for _, expected := range expectedStyles {
		if style == expected {
			return true
		}
	}
	return false
}

// GetExpectedStyles returns the expected case styles for a symbol kind in a language
func GetExpectedStyles(lang Language, kind SymbolKind) []CaseStyle {
	conv, ok := LanguageNamingConventions[lang]
	if !ok {
		return nil
	}
	return conv.ExpectedStyles[kind]
}

// DetectCaseStyle determines the case style of a name
func DetectCaseStyle(name string) CaseStyle {
	if len(name) == 0 {
		return CaseStyleUnknown
	}

	hasUnderscore := false
	hasHyphen := false
	hasUpperStart := name[0] >= 'A' && name[0] <= 'Z'
	hasLowerAfterUpper := false
	hasUpperAfterLower := false

	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '_' {
			hasUnderscore = true
		}
		if ch == '-' {
			hasHyphen = true
		}
		if i > 0 {
			prev := name[i-1]
			if prev >= 'a' && prev <= 'z' && ch >= 'A' && ch <= 'Z' {
				hasUpperAfterLower = true
			}
			if prev >= 'A' && prev <= 'Z' && ch >= 'a' && ch <= 'z' {
				hasLowerAfterUpper = true
			}
		}
	}

	switch {
	case hasUnderscore:
		return CaseStyleSnakeCase
	case hasHyphen:
		return CaseStyleKebabCase
	case hasUpperStart && (hasLowerAfterUpper || hasUpperAfterLower):
		return CaseStylePascalCase
	case !hasUpperStart && hasUpperAfterLower:
		return CaseStyleCamelCase
	default:
		return CaseStyleUnknown
	}
}
