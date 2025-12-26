package analysis

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/standardbeagle/lci/internal/types"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// BlockKey is a zero-allocation key type for code block comparison
// Replaces fmt.Sprintf("%d:%d:%d") in duplicate detection (5-10MB savings per analysis)
type BlockKey struct {
	FileID    types.FileID
	StartLine int
	EndLine   int
}

// DuplicateDetector finds code duplicates using multiple analysis techniques
type DuplicateDetector struct {
	// Hash-based duplicate tracking
	exactDuplicates      map[string][]CodeBlock // hash -> blocks
	structuralDuplicates map[string][]CodeBlock // normalized hash -> blocks
	semanticClusters     [][]CodeBlock          // semantic similarity clusters

	// Configuration
	minLines            int     // minimum lines to consider for duplication
	minTokens           int     // minimum tokens to consider
	similarityThreshold float64 // threshold for semantic similarity (0.0-1.0)

	mu sync.RWMutex
}

// CodeBlock represents a block of code that can be analyzed for duplication
type CodeBlock struct {
	FileID            types.FileID
	FilePath          string
	StartLine         int
	EndLine           int
	StartColumn       int
	EndColumn         int
	Content           string
	NormalizedContent string
	Tokens            []string
	ASTHash           string
	FunctionName      string
	BlockType         BlockType
	Size              BlockSize
}

// BlockType categorizes the type of code block
type BlockType string

const (
	BlockTypeFunction  BlockType = "function"
	BlockTypeMethod    BlockType = "method"
	BlockTypeClass     BlockType = "class"
	BlockTypeBlock     BlockType = "block"
	BlockTypeStatement BlockType = "statement"
)

// BlockSize contains size metrics for a code block
type BlockSize struct {
	Lines      int
	Tokens     int
	Characters int
	Complexity int // cyclomatic complexity
}

// DuplicateCluster represents a group of similar code blocks
type DuplicateCluster struct {
	ID         string
	Type       DuplicateType
	Blocks     []CodeBlock
	Similarity float64
	Impact     DuplicateImpact
	Suggestion string
}

// DuplicateType categorizes the kind of duplication
type DuplicateType string

const (
	DuplicateTypeExact      DuplicateType = "exact"      // Identical code
	DuplicateTypeStructural DuplicateType = "structural" // Same structure, different names
	DuplicateTypeSemantic   DuplicateType = "semantic"   // Similar functionality
)

// DuplicateImpact assesses the impact of duplication
type DuplicateImpact struct {
	Severity        ImpactSeverity
	LinesCount      int
	FilesCount      int
	Maintainability float64 // 0.0 (hard) to 1.0 (easy)
	Refactorable    bool
}

type ImpactSeverity string

const (
	ImpactLow      ImpactSeverity = "low"
	ImpactMedium   ImpactSeverity = "medium"
	ImpactHigh     ImpactSeverity = "high"
	ImpactCritical ImpactSeverity = "critical"
)

// NewDuplicateDetector creates a new duplicate detection system
func NewDuplicateDetector() *DuplicateDetector {
	return &DuplicateDetector{
		exactDuplicates:      make(map[string][]CodeBlock),
		structuralDuplicates: make(map[string][]CodeBlock),
		semanticClusters:     make([][]CodeBlock, 0),
		minLines:             5,   // minimum 5 lines
		minTokens:            20,  // minimum 20 tokens
		similarityThreshold:  0.8, // 80% similarity
	}
}

// AnalyzeFile analyzes a file for code blocks and adds them to duplicate detection
func (dd *DuplicateDetector) AnalyzeFile(fileID types.FileID, filePath string, content []byte, tree *tree_sitter.Tree) error {
	dd.mu.Lock()
	defer dd.mu.Unlock()

	blocks := dd.extractCodeBlocks(fileID, filePath, content, tree)

	for _, block := range blocks {
		// Skip blocks that are too small
		if block.Size.Lines < dd.minLines || block.Size.Tokens < dd.minTokens {
			continue
		}

		// Add to exact duplicate index
		exactHash := dd.computeExactHash(block.Content)
		dd.exactDuplicates[exactHash] = append(dd.exactDuplicates[exactHash], block)

		// Add to structural duplicate index
		structuralHash := dd.computeStructuralHash(block)
		dd.structuralDuplicates[structuralHash] = append(dd.structuralDuplicates[structuralHash], block)
	}

	return nil
}

// extractCodeBlocks extracts analyzable code blocks from the AST
func (dd *DuplicateDetector) extractCodeBlocks(fileID types.FileID, filePath string, content []byte, tree *tree_sitter.Tree) []CodeBlock {
	if tree == nil || tree.RootNode() == nil {
		return []CodeBlock{}
	}

	var blocks []CodeBlock
	dd.walkNode(tree.RootNode(), content, fileID, filePath, &blocks)
	return blocks
}

// walkNode recursively walks the AST to extract code blocks
func (dd *DuplicateDetector) walkNode(node *tree_sitter.Node, content []byte, fileID types.FileID, filePath string, blocks *[]CodeBlock) {
	if node == nil {
		return
	}

	nodeType := node.Kind()

	// Check if this node represents a code block worth analyzing
	if dd.isBlockNode(nodeType) && node.EndByte() > node.StartByte() {
		// Extract the text content of this block
		blockContent := content[node.StartByte():node.EndByte()]

		// Skip very small blocks (less than minimum lines)
		lines := strings.Count(string(blockContent), "\n") + 1
		if lines >= dd.minLines {
			// Calculate AST hash for this block
			astHash := dd.calculateASTHash(node)

			// Extract tokens for this block
			tokens := dd.tokenizeCode(string(blockContent))

			// Normalize content for structural comparison
			normalizedContent := dd.normalizeCode(string(blockContent))

			block := CodeBlock{
				FileID:            fileID,
				FilePath:          filePath,
				StartLine:         int(node.StartPosition().Row) + 1, // Convert to 1-based
				EndLine:           int(node.EndPosition().Row) + 1,
				StartColumn:       int(node.StartPosition().Column),
				EndColumn:         int(node.EndPosition().Column),
				Content:           string(blockContent),
				NormalizedContent: normalizedContent,
				Tokens:            tokens,
				ASTHash:           astHash,
			}

			*blocks = append(*blocks, block)
		}
	}

	// Recurse into children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		dd.walkNode(child, content, fileID, filePath, blocks)
	}
}

// isBlockNode determines if a node represents a code block worth analyzing
func (dd *DuplicateDetector) isBlockNode(nodeType string) bool {
	blockNodes := map[string]bool{
		"function_declaration": true,
		"method_definition":    true,
		"function_definition":  true,
		"class_declaration":    true,
		"class_definition":     true,
		"if_statement":         true,
		"for_statement":        true,
		"while_statement":      true,
		"switch_statement":     true,
		"try_statement":        true,
		"block_statement":      true,
		"compound_statement":   true,
	}

	return blockNodes[nodeType]
}

// normalizeCode normalizes code for structural comparison
func (dd *DuplicateDetector) normalizeCode(code string) string {
	// Remove whitespace and comments, normalize identifiers
	lines := strings.Split(code, "\n")
	var normalized []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}

		// Basic normalization - replace identifiers with placeholders
		normalized = append(normalized, dd.normalizeIdentifiers(line))
	}

	return strings.Join(normalized, "\n")
}

// normalizeIdentifiers replaces identifiers with generic placeholders
func (dd *DuplicateDetector) normalizeIdentifiers(line string) string {
	// This is a simplified normalization - in practice, you'd want more sophisticated
	// identifier replacement using the AST
	words := strings.Fields(line)
	var normalized []string

	for _, word := range words {
		// Keep keywords and operators, replace likely identifiers
		if dd.isKeyword(word) || dd.isOperator(word) {
			normalized = append(normalized, word)
		} else if dd.isLikelyIdentifier(word) {
			normalized = append(normalized, "ID")
		} else {
			normalized = append(normalized, word)
		}
	}

	return strings.Join(normalized, " ")
}

// tokenizeCode breaks code into tokens for analysis
func (dd *DuplicateDetector) tokenizeCode(code string) []string {
	// Basic tokenization - split on whitespace and common delimiters
	tokens := make([]string, 0)
	current := ""

	for _, char := range code {
		if dd.isDelimiter(char) {
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
			if !dd.isWhitespace(char) {
				tokens = append(tokens, string(char))
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		tokens = append(tokens, current)
	}

	return tokens
}

// computeExactHash computes hash for exact duplicate detection
func (dd *DuplicateDetector) computeExactHash(content string) string {
	// Remove leading/trailing whitespace but preserve internal structure
	normalized := strings.TrimSpace(content)
	hash := md5.Sum([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// computeStructuralHash computes hash for structural duplicate detection
func (dd *DuplicateDetector) computeStructuralHash(block CodeBlock) string {
	// Use normalized content for structural comparison
	hash := md5.Sum([]byte(block.NormalizedContent))
	return hex.EncodeToString(hash[:])
}

// Helper methods for code analysis
func (dd *DuplicateDetector) isKeyword(word string) bool {
	keywords := map[string]bool{
		"if": true, "else": true, "for": true, "while": true, "return": true,
		"function": true, "class": true, "var": true, "let": true, "const": true,
		"def": true, "import": true, "from": true, "and": true, "or": true,
		"true": true, "false": true, "null": true, "undefined": true,
	}
	return keywords[strings.ToLower(word)]
}

func (dd *DuplicateDetector) isOperator(word string) bool {
	operators := map[string]bool{
		"+": true, "-": true, "*": true, "/": true, "=": true, "==": true,
		"!=": true, "<": true, ">": true, "<=": true, ">=": true, "&&": true,
		"||": true, "!": true, "++": true, "--": true, "+=": true, "-=": true,
	}
	return operators[word]
}

func (dd *DuplicateDetector) isLikelyIdentifier(word string) bool {
	if len(word) == 0 {
		return false
	}

	// Check if it starts with a letter or underscore
	first := word[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_'
}

func (dd *DuplicateDetector) isDelimiter(char rune) bool {
	delimiters := "(){}[];,."
	return strings.ContainsRune(delimiters, char) || dd.isWhitespace(char)
}

func (dd *DuplicateDetector) isWhitespace(char rune) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}

func (dd *DuplicateDetector) calculateComplexity(node *tree_sitter.Node) int {
	// Basic cyclomatic complexity calculation
	complexity := 1 // base complexity

	dd.walkForComplexity(node, &complexity)

	return complexity
}

// calculateASTHash calculates a hash for the AST structure of a node
func (dd *DuplicateDetector) calculateASTHash(node *tree_sitter.Node) string {
	if node == nil {
		return ""
	}

	var astString strings.Builder
	dd.buildASTString(node, &astString, 0)

	hash := md5.Sum([]byte(astString.String()))
	return hex.EncodeToString(hash[:])
}

// buildASTString recursively builds a string representation of the AST
func (dd *DuplicateDetector) buildASTString(node *tree_sitter.Node, builder *strings.Builder, depth int) {
	if node == nil {
		return
	}

	// Add indentation
	for i := 0; i < depth; i++ {
		builder.WriteString("  ")
	}

	// Add node type
	builder.WriteString(node.Kind())
	builder.WriteString("\n")

	// Recurse into children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		dd.buildASTString(child, builder, depth+1)
	}
}

func (dd *DuplicateDetector) walkForComplexity(node *tree_sitter.Node, complexity *int) {
	nodeType := node.Kind()

	// Add complexity for decision points
	switch nodeType {
	case "if_statement", "while_statement", "for_statement", "switch_statement":
		*complexity++
	case "catch_clause", "except_clause":
		*complexity++
	case "conditional_expression":
		*complexity++
	}

	// Recursively process children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		dd.walkForComplexity(child, complexity)
	}
}

// GetDuplicates returns all detected duplicates organized by type
func (dd *DuplicateDetector) GetDuplicates() []DuplicateCluster {
	dd.mu.RLock()
	defer dd.mu.RUnlock()

	var clusters []DuplicateCluster

	// Process exact duplicates
	for hash, blocks := range dd.exactDuplicates {
		if len(blocks) > 1 {
			cluster := DuplicateCluster{
				ID:         hash,
				Type:       DuplicateTypeExact,
				Blocks:     blocks,
				Similarity: 1.0,
				Impact:     dd.calculateImpact(blocks),
				Suggestion: dd.generateSuggestion(DuplicateTypeExact, blocks),
			}
			clusters = append(clusters, cluster)
		}
	}

	// Process structural duplicates
	for hash, blocks := range dd.structuralDuplicates {
		if len(blocks) > 1 {
			// Check if these are already in exact duplicates
			if !dd.alreadyInExact(blocks) {
				cluster := DuplicateCluster{
					ID:         hash,
					Type:       DuplicateTypeStructural,
					Blocks:     blocks,
					Similarity: dd.calculateSimilarity(blocks),
					Impact:     dd.calculateImpact(blocks),
					Suggestion: dd.generateSuggestion(DuplicateTypeStructural, blocks),
				}
				clusters = append(clusters, cluster)
			}
		}
	}

	// Sort by impact severity
	sort.Slice(clusters, func(i, j int) bool {
		return dd.severityScore(clusters[i].Impact.Severity) > dd.severityScore(clusters[j].Impact.Severity)
	})

	return clusters
}

// calculateImpact assesses the impact of code duplication
func (dd *DuplicateDetector) calculateImpact(blocks []CodeBlock) DuplicateImpact {
	totalLines := 0
	filesMap := make(map[string]bool)

	for _, block := range blocks {
		totalLines += block.Size.Lines
		filesMap[block.FilePath] = true
	}

	filesCount := len(filesMap)

	// Determine severity based on size and distribution
	var severity ImpactSeverity
	switch {
	case totalLines > 200 || filesCount > 5:
		severity = ImpactCritical
	case totalLines > 100 || filesCount > 3:
		severity = ImpactHigh
	case totalLines > 50 || filesCount > 2:
		severity = ImpactMedium
	default:
		severity = ImpactLow
	}

	// Calculate maintainability score (inverse of complexity and size)
	avgComplexity := 0
	for _, block := range blocks {
		avgComplexity += block.Size.Complexity
	}
	avgComplexity /= len(blocks)

	maintainability := 1.0 / (1.0 + float64(avgComplexity)/10.0 + float64(totalLines)/1000.0)

	return DuplicateImpact{
		Severity:        severity,
		LinesCount:      totalLines,
		FilesCount:      filesCount,
		Maintainability: maintainability,
		Refactorable:    dd.isRefactorable(blocks),
	}
}

// Helper methods for analysis
func (dd *DuplicateDetector) alreadyInExact(blocks []CodeBlock) bool {
	for _, exactBlocks := range dd.exactDuplicates {
		if len(exactBlocks) > 1 && dd.sameBlocks(blocks, exactBlocks) {
			return true
		}
	}
	return false
}

func (dd *DuplicateDetector) sameBlocks(blocks1, blocks2 []CodeBlock) bool {
	if len(blocks1) != len(blocks2) {
		return false
	}

	// Create maps of block identifiers using struct keys (5-10MB savings)
	map1 := make(map[BlockKey]bool, len(blocks1))
	map2 := make(map[BlockKey]bool, len(blocks2))

	for _, block := range blocks1 {
		key := BlockKey{FileID: block.FileID, StartLine: block.StartLine, EndLine: block.EndLine}
		map1[key] = true
	}

	for _, block := range blocks2 {
		key := BlockKey{FileID: block.FileID, StartLine: block.StartLine, EndLine: block.EndLine}
		map2[key] = true
	}

	for key := range map1 {
		if !map2[key] {
			return false
		}
	}

	return true
}

func (dd *DuplicateDetector) calculateSimilarity(blocks []CodeBlock) float64 {
	if len(blocks) < 2 {
		return 0.0
	}

	// Calculate average similarity between all pairs
	totalSimilarity := 0.0
	pairs := 0

	for i := 0; i < len(blocks); i++ {
		for j := i + 1; j < len(blocks); j++ {
			similarity := dd.calculatePairSimilarity(blocks[i], blocks[j])
			totalSimilarity += similarity
			pairs++
		}
	}

	if pairs == 0 {
		return 0.0
	}

	return totalSimilarity / float64(pairs)
}

func (dd *DuplicateDetector) calculatePairSimilarity(block1, block2 CodeBlock) float64 {
	// Jaccard similarity based on tokens
	tokens1 := make(map[string]bool)
	tokens2 := make(map[string]bool)

	for _, token := range block1.Tokens {
		tokens1[token] = true
	}

	for _, token := range block2.Tokens {
		tokens2[token] = true
	}

	intersection := 0
	union := 0

	for token := range tokens1 {
		if tokens2[token] {
			intersection++
		}
		union++
	}

	for token := range tokens2 {
		if !tokens1[token] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func (dd *DuplicateDetector) generateSuggestion(duplicateType DuplicateType, blocks []CodeBlock) string {
	switch duplicateType {
	case DuplicateTypeExact:
		if len(blocks) == 2 {
			return "Extract identical code into a shared function or method"
		}
		return fmt.Sprintf("Extract identical code into a shared function used by %d locations", len(blocks))
	case DuplicateTypeStructural:
		return "Consider parameterizing the common structure to reduce duplication"
	case DuplicateTypeSemantic:
		return "Review for potential consolidation of similar functionality"
	default:
		return "Consider refactoring to reduce code duplication"
	}
}

func (dd *DuplicateDetector) isRefactorable(blocks []CodeBlock) bool {
	// Simple heuristic: blocks are refactorable if they're functions/methods
	// and not too complex
	for _, block := range blocks {
		if block.BlockType != BlockTypeFunction && block.BlockType != BlockTypeMethod {
			return false
		}
		if block.Size.Complexity > 15 {
			return false
		}
	}
	return true
}

func (dd *DuplicateDetector) severityScore(severity ImpactSeverity) int {
	switch severity {
	case ImpactCritical:
		return 4
	case ImpactHigh:
		return 3
	case ImpactMedium:
		return 2
	case ImpactLow:
		return 1
	default:
		return 0
	}
}

// Clear resets the duplicate detector state
func (dd *DuplicateDetector) Clear() {
	dd.mu.Lock()
	defer dd.mu.Unlock()

	dd.exactDuplicates = make(map[string][]CodeBlock)
	dd.structuralDuplicates = make(map[string][]CodeBlock)
	dd.semanticClusters = make([][]CodeBlock, 0)
}
