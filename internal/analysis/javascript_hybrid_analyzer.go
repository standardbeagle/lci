package analysis

import (
	"github.com/standardbeagle/lci/internal/types"
)

// JavaScriptHybridAnalyzer combines go-fAST (for ES5/CommonJS) with regex fallback (for ES6 modules)
// This provides accurate AST-based parsing for compatible code while maintaining compatibility
// with ES6 module syntax that go-fAST doesn't support
type JavaScriptHybridAnalyzer struct {
	goFastAnalyzer *JavaScriptGoFastAnalyzer
	regexAnalyzer  *JavaScriptAnalyzer
}

// NewJavaScriptHybridAnalyzer creates a new hybrid JavaScript analyzer
func NewJavaScriptHybridAnalyzer() *JavaScriptHybridAnalyzer {
	return &JavaScriptHybridAnalyzer{
		goFastAnalyzer: NewJavaScriptGoFastAnalyzer(),
		regexAnalyzer:  NewJavaScriptAnalyzer(),
	}
}

// GetLanguageName returns the language name
func (jha *JavaScriptHybridAnalyzer) GetLanguageName() string {
	return "javascript"
}

// ExtractSymbols tries go-fAST first, falls back to regex on parse failure
func (jha *JavaScriptHybridAnalyzer) ExtractSymbols(fileID types.FileID, content, filePath string) ([]*types.UniversalSymbolNode, error) {
	// Try go-fAST first for accurate AST-based parsing
	symbols, err := jha.goFastAnalyzer.ExtractSymbols(fileID, content, filePath)
	if err == nil && len(symbols) > 0 {
		return symbols, nil
	}

	// Fall back to regex for ES6 modules and other unsupported syntax
	return jha.regexAnalyzer.ExtractSymbols(fileID, content, filePath)
}

// AnalyzeExtends delegates to the appropriate analyzer
func (jha *JavaScriptHybridAnalyzer) AnalyzeExtends(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	// Both analyzers use similar extends analysis, use regex version
	return jha.regexAnalyzer.AnalyzeExtends(symbol, content, filePath)
}

// AnalyzeImplements delegates to the appropriate analyzer
func (jha *JavaScriptHybridAnalyzer) AnalyzeImplements(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	return jha.regexAnalyzer.AnalyzeImplements(symbol, content, filePath)
}

// AnalyzeContains delegates to the appropriate analyzer
func (jha *JavaScriptHybridAnalyzer) AnalyzeContains(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.CompositeSymbolID, error) {
	return jha.regexAnalyzer.AnalyzeContains(symbol, content, filePath)
}

// AnalyzeDependencies delegates to the appropriate analyzer
func (jha *JavaScriptHybridAnalyzer) AnalyzeDependencies(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.SymbolDependency, error) {
	return jha.regexAnalyzer.AnalyzeDependencies(symbol, content, filePath)
}

// AnalyzeCalls tries go-fAST first, falls back to regex
func (jha *JavaScriptHybridAnalyzer) AnalyzeCalls(symbol *types.UniversalSymbolNode, content, filePath string) ([]types.FunctionCall, error) {
	// Try go-fAST first
	calls, err := jha.goFastAnalyzer.AnalyzeCalls(symbol, content, filePath)
	if err == nil && len(calls) > 0 {
		return calls, nil
	}

	// Fall back to regex
	return jha.regexAnalyzer.AnalyzeCalls(symbol, content, filePath)
}
