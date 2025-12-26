package core

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/standardbeagle/lci/internal/types"
)

// Global cached regex patterns for imports
var (
	globalImportRegexes     map[string][]*regexp.Regexp
	globalImportRegexesOnce sync.Once
)

// ImportResolver provides language-agnostic heuristics for symbol resolution
// Uses practical approximations rather than full language server accuracy
type ImportResolver struct {
	// Import patterns by file extension (read-only after init)
	importPatterns map[string]*ImportPattern

	// File-to-file relationships discovered from imports
	// Built in single-threaded phase after indexing
	importGraph map[types.FileID][]ImportBinding
}

// ImportPattern defines how to extract imports for a language
type ImportPattern struct {
	// Regex patterns to find import statements
	ImportRegexes []*regexp.Regexp

	// How to extract the imported symbol and source
	SymbolExtractor func(importLine string) []ImportBinding

	// How to determine if a symbol is exported
	ExportChecker func(symbol types.Symbol) bool
}

// ImportBinding represents a symbol import relationship
type ImportBinding struct {
	ImportedName string // Name as imported (might be aliased)
	OriginalName string // Original symbol name in source file
	SourceFile   string // File path being imported from (relative)
	LineNumber   int    // Line where import occurs
	IsWildcard   bool   // true for "import *" or "use .*"
}

// FileImportData represents import data for a single file
// Used during parallel processing to collect imports without locking
type FileImportData struct {
	FileID   types.FileID
	Bindings []ImportBinding
}

// NewImportResolver creates a resolver with shared cached regex patterns
func NewImportResolver() *ImportResolver {
	// Initialize regex patterns once globally
	globalImportRegexesOnce.Do(func() {
		globalImportRegexes = map[string][]*regexp.Regexp{
			".go": {
				regexp.MustCompile(`import\s+"([^"]+)"`),             // import "package"
				regexp.MustCompile(`import\s+(\w+)\s+"([^"]+)"`),     // import alias "package"
				regexp.MustCompile(`(?s)import\s*\(\s*([^)]+)\s*\)`), // import ( ... ) multi-line
			},
			".js": {
				regexp.MustCompile(`import\s+\{([^}]+)\}\s+from\s+['"']([^'"]+)['"']`),     // import { A, B } from './file'
				regexp.MustCompile(`import\s+(\w+)\s+from\s+['"']([^'"]+)['"']`),           // import A from './file'
				regexp.MustCompile(`import\s+\*\s+as\s+(\w+)\s+from\s+['"']([^'"]+)['"']`), // import * as A from './file'
			},
			".py": {
				regexp.MustCompile(`from\s+([^\s]+)\s+import\s+([^#\n]+)`), // from module import A, B
				regexp.MustCompile(`import\s+([^\s#\n]+)`),                 // import module
			},
			".rs": {
				regexp.MustCompile(`use\s+([^;]+);`), // use crate::module::Symbol;
			},
		}
		// TypeScript shares JS patterns
		globalImportRegexes[".ts"] = globalImportRegexes[".js"]
		globalImportRegexes[".tsx"] = globalImportRegexes[".js"]
	})

	resolver := &ImportResolver{
		importPatterns: make(map[string]*ImportPattern),
		importGraph:    make(map[types.FileID][]ImportBinding),
	}

	// Create ImportPattern structs with shared regexes but instance-specific function pointers
	resolver.importPatterns[".go"] = &ImportPattern{
		ImportRegexes:   globalImportRegexes[".go"],
		SymbolExtractor: resolver.extractGoImports,
		ExportChecker:   resolver.isGoExported,
	}

	jsPattern := &ImportPattern{
		ImportRegexes:   globalImportRegexes[".js"],
		SymbolExtractor: resolver.extractJSImports,
		ExportChecker:   resolver.isJSExported,
	}
	resolver.importPatterns[".js"] = jsPattern
	resolver.importPatterns[".ts"] = jsPattern
	resolver.importPatterns[".tsx"] = jsPattern

	resolver.importPatterns[".py"] = &ImportPattern{
		ImportRegexes:   globalImportRegexes[".py"],
		SymbolExtractor: resolver.extractPythonImports,
		ExportChecker:   resolver.isPythonExported,
	}

	resolver.importPatterns[".rs"] = &ImportPattern{
		ImportRegexes:   globalImportRegexes[".rs"],
		SymbolExtractor: resolver.extractRustImports,
		ExportChecker:   resolver.isRustExported,
	}

	return resolver
}

// ExtractFileImports analyzes a file's imports and returns the import data
// This is lock-free and can be called safely from parallel goroutines
func (ir *ImportResolver) ExtractFileImports(fileID types.FileID, filePath string, content []byte) *FileImportData {
	ext := strings.ToLower(filepath.Ext(filePath))
	pattern, exists := ir.importPatterns[ext]
	if !exists {
		return nil // Unsupported language
	}

	contentStr := string(content)
	// Use optimized line splitting with pre-counted capacity
	lines := SplitLinesWithCapacity(content)

	var bindings []ImportBinding

	// Apply all regex patterns for this language
	for _, regex := range pattern.ImportRegexes {
		matches := regex.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			// Extract bindings using language-specific logic
			lineBindings := pattern.SymbolExtractor(match[0])
			for i := range lineBindings {
				// Find which line this import is on
				for lineNum, line := range lines {
					if strings.Contains(line, match[0]) {
						lineBindings[i].LineNumber = lineNum + 1
						break
					}
				}
			}
			bindings = append(bindings, lineBindings...)
		}
	}

	return &FileImportData{
		FileID:   fileID,
		Bindings: bindings,
	}
}

// Language-specific import extractors

// extractGoImports parses Go import statements and returns ImportBindings
func (ir *ImportResolver) extractGoImports(importMatch string) []ImportBinding {
	var bindings []ImportBinding

	// Handle different Go import patterns
	if strings.Contains(importMatch, "(") && strings.Contains(importMatch, ")") {
		// Multi-line import block: import ( ... )
		start := strings.Index(importMatch, "(")
		end := strings.LastIndex(importMatch, ")")
		if start >= 0 && end > start {
			content := importMatch[start+1 : end]
			// Use LineScanner for zero-allocation iteration
			ForEachLine([]byte(content), func(lineBytes []byte, _ int) bool {
				line := strings.TrimSpace(string(lineBytes))
				if line == "" || strings.HasPrefix(line, "//") {
					return true // continue
				}
				binding := ir.parseGoImportLine(line)
				if binding != nil {
					bindings = append(bindings, *binding)
				}
				return true
			})
		}
	} else {
		// Single import: import "package" or import alias "package"
		line := strings.TrimPrefix(importMatch, "import")
		line = strings.TrimSpace(line)
		binding := ir.parseGoImportLine(line)
		if binding != nil {
			bindings = append(bindings, *binding)
		}
	}

	return bindings
}

// parseGoImportLine parses a single Go import line
func (ir *ImportResolver) parseGoImportLine(line string) *ImportBinding {
	line = strings.TrimSpace(line)

	// Check for alias: alias "package"
	parts := strings.Fields(line)
	if len(parts) == 2 {
		alias := parts[0]
		packagePath := strings.Trim(parts[1], `"`)
		return &ImportBinding{
			ImportedName: alias,
			OriginalName: alias,
			SourceFile:   packagePath,
		}
	} else if len(parts) == 1 {
		// No alias: "package"
		packagePath := strings.Trim(parts[0], `"`)
		packageName := packagePath
		if idx := strings.LastIndex(packagePath, "/"); idx >= 0 {
			packageName = packagePath[idx+1:]
		}
		return &ImportBinding{
			ImportedName: packageName,
			OriginalName: packageName,
			SourceFile:   packagePath,
		}
	}

	return nil
}

// extractJSImports parses JavaScript/TypeScript import statements
func (ir *ImportResolver) extractJSImports(importMatch string) []ImportBinding {
	var bindings []ImportBinding

	fromIndex := strings.Index(importMatch, "from")
	if fromIndex <= 0 {
		return bindings
	}

	sourceFile := strings.TrimSpace(importMatch[fromIndex+4:])
	sourceFile = strings.Trim(sourceFile, `'";`)

	importPart := strings.TrimSpace(importMatch[:fromIndex])
	importPart = strings.TrimPrefix(importPart, "import")
	importPart = strings.TrimSpace(importPart)

	// Parse different JS import patterns
	if strings.Contains(importPart, "{") && strings.Contains(importPart, "}") {
		// import { A, B } from './file'
		start := strings.Index(importPart, "{")
		end := strings.Index(importPart, "}")
		if start >= 0 && end > start {
			namedImports := importPart[start+1 : end]
			for _, imp := range strings.Split(namedImports, ",") {
				imp = strings.TrimSpace(imp)
				if imp == "" {
					continue
				}

				// Handle "name as alias" using strings.Cut (cleaner than Split)
				if originalName, importedName, ok := strings.Cut(imp, " as "); ok {
					bindings = append(bindings, ImportBinding{
						ImportedName: strings.TrimSpace(importedName),
						OriginalName: strings.TrimSpace(originalName),
						SourceFile:   sourceFile,
					})
				} else {
					bindings = append(bindings, ImportBinding{
						ImportedName: imp,
						OriginalName: imp,
						SourceFile:   sourceFile,
					})
				}
			}
		}
	} else if strings.Contains(importPart, "* as ") {
		// import * as utils from './file'
		if _, alias, ok := strings.Cut(importPart, "* as "); ok {
			bindings = append(bindings, ImportBinding{
				ImportedName: strings.TrimSpace(alias),
				OriginalName: "*",
				SourceFile:   sourceFile,
			})
		}
	} else {
		// import React from './file' - default import
		defaultName := strings.TrimSpace(importPart)
		if defaultName != "" {
			bindings = append(bindings, ImportBinding{
				ImportedName: defaultName,
				OriginalName: defaultName,
				SourceFile:   sourceFile,
			})
		}
	}

	return bindings
}

// extractPythonImports parses Python import statements
func (ir *ImportResolver) extractPythonImports(importMatch string) []ImportBinding {
	var bindings []ImportBinding

	if strings.HasPrefix(importMatch, "from ") {
		// from module import A, B
		// Use strings.Cut instead of strings.Split for cleaner code
		if fromPart, importsPart, ok := strings.Cut(importMatch, " import "); ok {
			sourceFile := strings.TrimSpace(strings.TrimPrefix(fromPart, "from"))

			for _, imp := range strings.Split(importsPart, ",") {
				imp = strings.TrimSpace(imp)
				if imp == "" {
					continue
				}

				// Handle "name as alias" using strings.Cut
				if originalName, importedName, ok := strings.Cut(imp, " as "); ok {
					bindings = append(bindings, ImportBinding{
						ImportedName: strings.TrimSpace(importedName),
						OriginalName: strings.TrimSpace(originalName),
						SourceFile:   sourceFile,
					})
				} else {
					bindings = append(bindings, ImportBinding{
						ImportedName: imp,
						OriginalName: imp,
						SourceFile:   sourceFile,
					})
				}
			}
		}
	} else if strings.HasPrefix(importMatch, "import ") {
		// import module
		module := strings.TrimSpace(strings.TrimPrefix(importMatch, "import"))
		bindings = append(bindings, ImportBinding{
			ImportedName: module,
			OriginalName: module,
			SourceFile:   module,
		})
	}

	return bindings
}

// extractRustImports parses Rust use statements
func (ir *ImportResolver) extractRustImports(importMatch string) []ImportBinding {
	var bindings []ImportBinding

	// use crate::module::Symbol;
	// use std::collections::HashMap;
	useStatement := strings.TrimSpace(strings.TrimPrefix(importMatch, "use"))
	useStatement = strings.TrimSuffix(useStatement, ";")

	// Handle braced imports: use std::{A, B};
	if strings.Contains(useStatement, "{") && strings.Contains(useStatement, "}") {
		// Extract base path and symbols
		start := strings.Index(useStatement, "{")
		end := strings.Index(useStatement, "}")
		if start >= 0 && end > start {
			basePath := strings.TrimSpace(useStatement[:start])
			basePath = strings.TrimSuffix(basePath, "::")

			symbols := useStatement[start+1 : end]
			for _, symbol := range strings.Split(symbols, ",") {
				symbol = strings.TrimSpace(symbol)
				if symbol == "" {
					continue
				}

				bindings = append(bindings, ImportBinding{
					ImportedName: symbol,
					OriginalName: symbol,
					SourceFile:   basePath,
				})
			}
		}
	} else {
		// Single import: use std::collections::HashMap;
		// Use LastIndex to avoid full Split allocation
		if lastSep := strings.LastIndex(useStatement, "::"); lastSep >= 0 {
			symbol := useStatement[lastSep+2:]
			sourcePath := useStatement[:lastSep]

			bindings = append(bindings, ImportBinding{
				ImportedName: symbol,
				OriginalName: symbol,
				SourceFile:   sourcePath,
			})
		} else if useStatement != "" {
			// No path separator, just the symbol itself
			bindings = append(bindings, ImportBinding{
				ImportedName: useStatement,
				OriginalName: useStatement,
				SourceFile:   "",
			})
		}
	}

	return bindings
}

// Export checker functions

func (ir *ImportResolver) isGoExported(symbol types.Symbol) bool {
	return len(symbol.Name) > 0 && symbol.Name[0] >= 'A' && symbol.Name[0] <= 'Z'
}

func (ir *ImportResolver) isJSExported(symbol types.Symbol) bool {
	return true // JavaScript doesn't have built-in export visibility
}

func (ir *ImportResolver) isPythonExported(symbol types.Symbol) bool {
	return !strings.HasPrefix(symbol.Name, "_") // Python convention
}

func (ir *ImportResolver) isRustExported(symbol types.Symbol) bool {
	return len(symbol.Name) > 0 && symbol.Name[0] >= 'A' && symbol.Name[0] <= 'Z' // Rust uses pub keyword, hard to detect here
}

// BuildImportGraph builds the import graph from collected import data
// This should be called in a single-threaded phase after all files are processed
func (ir *ImportResolver) BuildImportGraph(importData []*FileImportData) {
	// Clear existing graph
	ir.importGraph = make(map[types.FileID][]ImportBinding)

	// Build graph from collected data
	for _, data := range importData {
		if data != nil && len(data.Bindings) > 0 {
			ir.importGraph[data.FileID] = data.Bindings
		}
	}
}

// ResolveSymbolReference attempts to resolve which specific symbol a reference points to
func (ir *ImportResolver) ResolveSymbolReference(refFileID types.FileID, referencedName string, candidates []types.SymbolID, symbolLookup func(types.SymbolID) *types.EnhancedSymbol) types.SymbolID {
	// Strategy 1: Check if symbol is imported in this file
	if bindings, exists := ir.importGraph[refFileID]; exists {
		for _, binding := range bindings {
			if binding.ImportedName == referencedName || binding.OriginalName == referencedName {
				// Find candidate symbol from the imported source file
				// Since we have name match from imports, prioritize these candidates
				// The import resolver tracks which files import which symbols
				for _, candidateID := range candidates {
					if symbol := symbolLookup(candidateID); symbol != nil {
						// If we found a matching symbol from this import binding, return it
						// This is high-confidence because the name matches and it's explicitly imported
						return candidateID
					}
				}
			}
		}
	}

	// Strategy 2: Prefer symbols from same file/package
	for _, candidateID := range candidates {
		if symbol := symbolLookup(candidateID); symbol != nil {
			if symbol.Symbol.FileID == refFileID {
				return candidateID // Same file - highest confidence
			}
		}
	}

	// Strategy 3: Prefer exported symbols (more likely to be referenced)
	for _, candidateID := range candidates {
		if symbol := symbolLookup(candidateID); symbol != nil {
			// Use the IsExported field which is language-aware
			// This handles different export conventions across languages:
			// - Go: Capital first letter
			// - JavaScript/TypeScript: export keyword (set during parsing)
			// - Python: Not starting with _ (set during parsing)
			// - Java/C#/etc.: public/protected modifiers (set during parsing)
			if symbol.IsExported {
				return candidateID
			}
		}
	}

	// Strategy 4: Fallback to first candidate
	if len(candidates) > 0 {
		return candidates[0]
	}

	return 0
}

// filePathMatches checks if a file path matches an import path
// Handles both absolute and relative paths by comparing path suffixes
func (ir *ImportResolver) filePathMatches(filePath, importPath string) bool {
	// Direct match
	if filePath == importPath {
		return true
	}

	// Suffix match (handles relative vs absolute paths)
	// e.g., "/project/src/utils/helper.js" matches "utils/helper.js"
	if len(filePath) >= len(importPath) {
		suffix := filePath[len(filePath)-len(importPath):]
		if suffix == importPath {
			return true
		}
	}

	// Reverse suffix match
	// e.g., "utils/helper.js" matches "/project/src/utils/helper.js"
	if len(importPath) >= len(filePath) {
		suffix := importPath[len(importPath)-len(filePath):]
		if suffix == filePath {
			return true
		}
	}

	// Check if import path is a substring (for module resolution)
	if strings.Contains(filePath, importPath) || strings.Contains(importPath, filePath) {
		// Only match if it ends with the same file/path component
		fileBase := filePath[strings.LastIndex(filePath, "/")+1:]
		importBase := importPath[strings.LastIndex(importPath, "/")+1:]
		return fileBase == importBase
	}

	return false
}

// Clear removes all import data
func (ir *ImportResolver) Clear() {
	// Clear the import graph
	ir.importGraph = make(map[types.FileID][]ImportBinding)
}

// RemoveFile removes import data for a specific file
func (ir *ImportResolver) RemoveFile(fileID types.FileID) {
	delete(ir.importGraph, fileID)
}

// Shutdown performs graceful shutdown with resource cleanup
func (ir *ImportResolver) Shutdown() error {
	// Clear all data
	ir.Clear()
	return nil
}
