package core

import (
	"fmt"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	"github.com/standardbeagle/lci/internal/types"
)

// ASTStore maintains per-file AST trees for Tree-sitter queries
// Optimized for incremental updates - easy to add/remove/update individual files
// Content is accessed from FileContentStore to avoid duplication
//
// LOCK-FREE DESIGN: During indexing, this store follows single-writer pattern where
// only the integrator modifies the maps while processors read content. We eliminate
// mutexes and use atomic snapshots for safe concurrent access.
type ASTStore struct {
	// Per-file storage for easy incremental updates
	astsByFile     map[types.FileID]*tree_sitter.Tree // FileID -> AST tree
	pathByFile     map[types.FileID]string            // FileID -> file path
	languageByFile map[types.FileID]string            // FileID -> language extension (.go, .js, etc.)

	// Reference to FileContentStore for content access (no duplication)
	fileContentStore *FileContentStore

	// LOCK-FREE: Single-writer constraint during indexing eliminates need for mutexes
	// During indexing: Only integrator writes, processors read content from FileContentStore
	// After indexing: Maps are immutable for query operations
}

// NewASTStore creates a new AST store with reference to FileContentStore
func NewASTStore(fileContentStore *FileContentStore) *ASTStore {
	return &ASTStore{
		astsByFile:       make(map[types.FileID]*tree_sitter.Tree),
		pathByFile:       make(map[types.FileID]string),
		languageByFile:   make(map[types.FileID]string),
		fileContentStore: fileContentStore,
	}
}

// StoreAST stores an AST tree for a file (content comes from FileContentStore, no duplication)
// LOCK-FREE: Single-writer constraint during indexing - only integrator calls this
func (as *ASTStore) StoreAST(fileID types.FileID, ast *tree_sitter.Tree, path string, language string) {
	// Clean up any existing AST for this file
	if existingAST, exists := as.astsByFile[fileID]; exists && existingAST != nil {
		existingAST.Close()
	}

	as.astsByFile[fileID] = ast
	as.pathByFile[fileID] = path
	as.languageByFile[fileID] = language
	// Note: content is accessed from FileContentStore when needed, no duplication
}

// GetAST retrieves the AST for a file (content accessed from FileContentStore)
// LOCK-FREE: Maps are immutable after indexing, safe to read without synchronization
func (as *ASTStore) GetAST(fileID types.FileID) (*tree_sitter.Tree, []byte, string, string, bool) {
	ast, astExists := as.astsByFile[fileID]
	path, pathExists := as.pathByFile[fileID]
	language, langExists := as.languageByFile[fileID]

	if astExists && pathExists && langExists {
		// Get content from FileContentStore (no duplication)
		content, exists := as.fileContentStore.GetContent(fileID)
		if exists {
			return ast, content, path, language, true
		}
	}

	return nil, nil, "", "", false
}

// RemoveFile removes all data for a file (for incremental updates)
// LOCK-FREE: Single-writer constraint during indexing - only integrator calls this
func (as *ASTStore) RemoveFile(fileID types.FileID) {
	// Clean up AST memory
	if ast, exists := as.astsByFile[fileID]; exists && ast != nil {
		ast.Close()
	}

	delete(as.astsByFile, fileID)
	delete(as.pathByFile, fileID)
	delete(as.languageByFile, fileID)
	// Note: content is managed by FileContentStore, not duplicated here
}

// GetAllFiles returns all file IDs that have ASTs stored
// LOCK-FREE: Maps are immutable after indexing, safe to read without synchronization
func (as *ASTStore) GetAllFiles() []types.FileID {
	files := make([]types.FileID, 0, len(as.astsByFile))
	for fileID := range as.astsByFile {
		files = append(files, fileID)
	}
	return files
}

// GetFileCount returns the number of files with stored ASTs  
// LOCK-FREE: Maps are immutable after indexing, safe to read without synchronization
func (as *ASTStore) GetFileCount() int {
	return len(as.astsByFile)
}

// Clear removes all stored ASTs and associated data
// LOCK-FREE: Single-writer constraint - only called during shutdown/reset operations
func (as *ASTStore) Clear() {
	// Clean up all AST memory
	for _, ast := range as.astsByFile {
		if ast != nil {
			ast.Close()
		}
	}

	as.astsByFile = make(map[types.FileID]*tree_sitter.Tree)
	as.pathByFile = make(map[types.FileID]string)
	as.languageByFile = make(map[types.FileID]string)
	// Note: content is managed by FileContentStore, not duplicated here
}

// Shutdown performs graceful shutdown with resource cleanup
func (as *ASTStore) Shutdown() error {
	// Clear all data which includes proper AST cleanup
	as.Clear()
	return nil
}

// GetMemoryStats returns memory usage statistics (content not duplicated)
// LOCK-FREE: Maps are immutable after indexing, safe to read without synchronization
func (as *ASTStore) GetMemoryStats() map[string]interface{} {
	// Note: Content size not calculated here since it's managed by FileContentStore
	// This prevents the double counting that was causing memory bloom
	return map[string]interface{}{
		"file_count":           len(as.astsByFile),
		"ast_trees":            len(as.astsByFile),
		"note":                 "content_bytes managed by FileContentStore, no duplication",
	}
}

// QueryResult represents the result of a Tree-sitter query
type QueryResult struct {
	FileID   types.FileID `json:"file_id"`
	FilePath string       `json:"file_path"`
	Language string       `json:"language"`
	Matches  []QueryMatch `json:"matches"`
}

// QueryMatch represents a single match from a Tree-sitter query
type QueryMatch struct {
	CaptureNames []string          `json:"capture_names"` // Names of captured nodes (@function.name, @call.target, etc.)
	Captures     []QueryNode       `json:"captures"`      // The captured nodes
	StartByte    uint              `json:"start_byte"`
	EndByte      uint              `json:"end_byte"`
	StartPoint   tree_sitter.Point `json:"start_point"`
	EndPoint     tree_sitter.Point `json:"end_point"`
	Text         string            `json:"text"` // The matched text
}

// QueryNode represents a captured node in a Tree-sitter query
type QueryNode struct {
	Name       string            `json:"name"` // Capture name (@function.name, @call.target, etc.)
	Type       string            `json:"type"` // Node type (identifier, function_declaration, etc.)
	Text       string            `json:"text"` // Node text content
	StartByte  uint              `json:"start_byte"`
	EndByte    uint              `json:"end_byte"`
	StartPoint tree_sitter.Point `json:"start_point"`
	EndPoint   tree_sitter.Point `json:"end_point"`
}

// ExecuteQuery runs a Tree-sitter query across all files or specific files
// LOCK-FREE: Maps are immutable after indexing, safe to read without synchronization
func (as *ASTStore) ExecuteQuery(queryStr string, fileIDs []types.FileID, language string) ([]QueryResult, error) {
	// If no specific files requested, query all files
	targetFiles := fileIDs
	if len(targetFiles) == 0 {
		targetFiles = as.GetAllFiles()
	}

	var results []QueryResult

	for _, fileID := range targetFiles {
		ast, content, path, fileLang, exists := as.getASTUnsafe(fileID)
		if !exists || ast == nil || content == nil {
			continue
		}

		// Skip if language filter specified and doesn't match
		if language != "" && fileLang != language {
			continue
		}

		// Execute query on this file's AST
		matches, err := as.executeQueryOnAST(ast, content, queryStr, fileLang)
		if err != nil {
			// If we're only querying one file, return the error
			if len(targetFiles) == 1 {
				return nil, fmt.Errorf("failed to execute query on file %s: %w", path, err)
			}
			// Otherwise log error but continue with other files
			continue
		}

		if len(matches) > 0 {
			results = append(results, QueryResult{
				FileID:   fileID,
				FilePath: path,
				Language: fileLang,
				Matches:  matches,
			})
		}
	}

	return results, nil
}

// executeQueryOnAST runs a query on a single AST
func (as *ASTStore) executeQueryOnAST(ast *tree_sitter.Tree, content []byte, queryStr string, language string) ([]QueryMatch, error) {
	// Get the language based on file extension
	lang := as.getLanguageForExtension(language)
	if lang == nil {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// Create a query from the query string (official API uses string, not []byte)
	query, queryErr := tree_sitter.NewQuery(lang, queryStr)
	if queryErr != nil {
		return nil, fmt.Errorf("invalid query syntax: %v", queryErr.Message)
	}
	defer query.Close()

	// Create a query cursor
	qc := tree_sitter.NewQueryCursor()
	defer qc.Close()

	// Execute the query using the official API
	queryMatches := qc.Matches(query, ast.RootNode(), content)

	var matches []QueryMatch
	captureNames := query.CaptureNames()

	// Iterate through all matches
	for {
		match := queryMatches.Next()
		if match == nil {
			break
		}

		// Collect all captures for this match
		var captures []QueryNode
		var matchCaptureNames []string
		var startByte, endByte uint
		var startPoint, endPoint tree_sitter.Point
		var matchText string

		// Track the overall match bounds
		firstCapture := true

		for _, capture := range match.Captures {
			node := capture.Node

			// Get capture name using index
			var captureName string
			if int(capture.Index) < len(captureNames) {
				captureName = captureNames[capture.Index]
			}
			matchCaptureNames = append(matchCaptureNames, captureName)

			// Extract node text
			nodeText := string(content[node.StartByte():node.EndByte()])

			// Create capture node
			captures = append(captures, QueryNode{
				Name:       captureName,
				Type:       node.Kind(),
				Text:       nodeText,
				StartByte:  node.StartByte(),
				EndByte:    node.EndByte(),
				StartPoint: node.StartPosition(),
				EndPoint:   node.EndPosition(),
			})

			// Update match bounds
			if firstCapture {
				startByte = node.StartByte()
				endByte = node.EndByte()
				startPoint = node.StartPosition()
				endPoint = node.EndPosition()
				matchText = nodeText
				firstCapture = false
			} else {
				// Extend bounds to include all captures
				if node.StartByte() < startByte {
					startByte = node.StartByte()
					startPoint = node.StartPosition()
				}
				if node.EndByte() > endByte {
					endByte = node.EndByte()
					endPoint = node.EndPosition()
				}
			}
		}

		// Extract the full match text if it spans multiple captures
		if endByte > startByte {
			matchText = string(content[startByte:endByte])
		}

		// Create the match result
		if len(captures) > 0 {
			matches = append(matches, QueryMatch{
				CaptureNames: matchCaptureNames,
				Captures:     captures,
				StartByte:    startByte,
				EndByte:      endByte,
				StartPoint:   startPoint,
				EndPoint:     endPoint,
				Text:         matchText,
			})
		}
	}

	return matches, nil
}

// getASTUnsafe is the non-locking version of GetAST for internal use (content from FileContentStore)
func (as *ASTStore) getASTUnsafe(fileID types.FileID) (*tree_sitter.Tree, []byte, string, string, bool) {
	ast, astExists := as.astsByFile[fileID]
	path, pathExists := as.pathByFile[fileID]
	language, langExists := as.languageByFile[fileID]

	if astExists && pathExists && langExists {
		// Get content from FileContentStore (no duplication)
		content, exists := as.fileContentStore.GetContent(fileID)
		if exists {
			return ast, content, path, language, true
		}
	}

	return nil, nil, "", "", false
}

// GetSupportedLanguages returns the languages currently indexed
// LOCK-FREE: Maps are immutable after indexing, safe to read without synchronization
func (as *ASTStore) GetSupportedLanguages() []string {
	languages := make(map[string]bool)
	for _, lang := range as.languageByFile {
		languages[lang] = true
	}

	result := make([]string, 0, len(languages))
	for lang := range languages {
		result = append(result, lang)
	}
	return result
}

// getLanguageForExtension returns the Tree-sitter language for a file extension
func (as *ASTStore) getLanguageForExtension(ext string) *tree_sitter.Language {
	// Import the language parsers we need
	var languagePtr unsafe.Pointer

	switch ext {
	case ".go":
		languagePtr = tree_sitter_go.Language()
	case ".js", ".jsx":
		languagePtr = tree_sitter_javascript.Language()
	case ".ts", ".tsx":
		languagePtr = tree_sitter_javascript.Language() // For now, use JS parser for TS
	case ".py":
		languagePtr = tree_sitter_python.Language()
	default:
		return nil
	}

	if languagePtr == nil {
		return nil
	}

	return tree_sitter.NewLanguage(languagePtr)
}
