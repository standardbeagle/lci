package mcp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// ExpansionEngine handles expansion of context references into full hydrated context
type ExpansionEngine struct {
	refTracker           *core.ReferenceTracker
	fileService          *core.FileService
	index                interface{} // MasterIndex for lookups
	sideEffectPropagator interface{} // SideEffectPropagator for purity analysis
}

// NewExpansionEngine creates a new expansion engine
func NewExpansionEngine(refTracker *core.ReferenceTracker, index interface{}) *ExpansionEngine {
	engine := &ExpansionEngine{
		refTracker:  refTracker,
		fileService: core.NewFileService(),
		index:       index,
	}

	// Try to get the side effect propagator from the index
	if index != nil {
		if indexWithPropagator, ok := index.(interface {
			GetSideEffectPropagator() *core.SideEffectPropagator
		}); ok {
			engine.sideEffectPropagator = indexWithPropagator.GetSideEffectPropagator()
		}
	}

	return engine
}

// ParseExpansionDirective parses an expansion directive like "callers:2"
func ParseExpansionDirective(directive string) (string, int) {
	parts := strings.Split(directive, ":")
	directiveType := parts[0]
	depth := 1 // Default depth

	if len(parts) > 1 {
		// Try to parse depth
		var d int
		if _, err := fmt.Sscanf(parts[1], "%d", &d); err == nil && d > 0 {
			depth = d
		}
	}

	return directiveType, depth
}

// HydrateReference resolves a reference into source code
func (e *ExpansionEngine) HydrateReference(
	ctx context.Context,
	ref types.ContextRef,
	format types.FormatType,
	projectRoot string,
) (*types.HydratedRef, int, error) {
	hydratedRef := &types.HydratedRef{
		File:     ref.F,
		Symbol:   ref.S,
		Role:     ref.Role,
		Note:     ref.Note,
		Expanded: make(map[string][]types.HydratedRef),
	}

	// Resolve file path
	filePath := ref.F
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(projectRoot, ref.F)
	}

	// Case 1: Symbol name provided - look up symbol and get source
	if ref.S != "" {
		// Try to get enhanced symbol from tracker
		if e.refTracker != nil {
			// We'll need to search for the symbol by name in the file
			// For now, use a simple approach: read file and find symbol
			source, lines, symbolInfo, err := e.extractSymbolSource(filePath, ref.S, ref.L, format)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to extract symbol %s: %w", ref.S, err)
			}

			hydratedRef.Source = source
			hydratedRef.Lines = lines
			if symbolInfo != nil {
				hydratedRef.SymbolType = symbolInfo.SymbolType
				hydratedRef.Signature = symbolInfo.Signature
				hydratedRef.IsExported = symbolInfo.IsExported
			}
		} else {
			// Fallback: extract by line range if provided
			if ref.L != nil {
				source, err := e.extractSourceByLines(filePath, ref.L.Start, ref.L.End)
				if err != nil {
					return nil, 0, fmt.Errorf("failed to extract lines: %w", err)
				}
				hydratedRef.Source = source
				hydratedRef.Lines = *ref.L
			} else {
				return nil, 0, errors.New("no reference tracker and no line range - cannot resolve symbol")
			}
		}
	} else if ref.L != nil {
		// Case 2: Only line range provided
		source, err := e.extractSourceByLines(filePath, ref.L.Start, ref.L.End)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to extract lines: %w", err)
		}
		hydratedRef.Source = source
		hydratedRef.Lines = *ref.L
	} else {
		return nil, 0, errors.New("reference must have either symbol name or line range")
	}

	// Estimate tokens (rough heuristic: 1 token per 4 characters)
	tokens := len(hydratedRef.Source) / 4

	return hydratedRef, tokens, nil
}

// SymbolInfo contains metadata about a symbol
type SymbolInfo struct {
	SymbolType string
	Signature  string
	IsExported bool
	StartLine  int
	EndLine    int
}

// extractSymbolSource extracts source code for a symbol
func (e *ExpansionEngine) extractSymbolSource(
	filePath string,
	symbolName string,
	lineRangeHint *types.LineRange,
	format types.FormatType,
) (string, types.LineRange, *SymbolInfo, error) {
	// If line range hint provided, use it directly
	if lineRangeHint != nil {
		source, err := e.extractSourceByLines(filePath, lineRangeHint.Start, lineRangeHint.End)
		if err != nil {
			return "", types.LineRange{}, nil, err
		}

		// Parse symbol info from source (simplified)
		info := &SymbolInfo{
			StartLine: lineRangeHint.Start,
			EndLine:   lineRangeHint.End,
		}

		// Try to extract signature from first line
		lines := strings.Split(source, "\n")
		if len(lines) > 0 {
			info.Signature = strings.TrimSpace(lines[0])
		}

		return source, *lineRangeHint, info, nil
	}

	// Otherwise, need to search for symbol in file
	// For now, read entire file and search (TODO: optimize with index)
	fileContent, err := e.fileService.ReadFile(filePath)
	if err != nil {
		return "", types.LineRange{}, nil, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(fileContent), "\n")

	// Simple search: look for lines containing the symbol name
	// This is a placeholder - ideally use tree-sitter AST from index
	for i, line := range lines {
		if strings.Contains(line, symbolName) && (strings.Contains(line, "func") || strings.Contains(line, "type") || strings.Contains(line, "class")) {
			// Found potential symbol definition
			startLine := i + 1 // 1-indexed

			// Heuristic: extract until next top-level definition or end
			endLine := startLine
			for j := i + 1; j < len(lines); j++ {
				line := strings.TrimSpace(lines[j])
				// Stop at next top-level definition
				if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") || strings.HasPrefix(line, "class ") {
					break
				}
				endLine = j + 1
			}

			source, err := e.extractSourceByLines(filePath, startLine, endLine)
			if err != nil {
				return "", types.LineRange{}, nil, err
			}

			info := &SymbolInfo{
				StartLine: startLine,
				EndLine:   endLine,
				Signature: strings.TrimSpace(lines[i]),
			}

			return source, types.LineRange{Start: startLine, End: endLine}, info, nil
		}
	}

	return "", types.LineRange{}, nil, fmt.Errorf("symbol %s not found in %s", symbolName, filePath)
}

// extractSourceByLines extracts source code by line range
func (e *ExpansionEngine) extractSourceByLines(filePath string, startLine, endLine int) (string, error) {
	fileContent, err := e.fileService.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(fileContent), "\n")

	// Validate line range
	if startLine < 1 || startLine > len(lines) {
		return "", fmt.Errorf("start line %d out of range (file has %d lines)", startLine, len(lines))
	}
	if endLine < startLine || endLine > len(lines) {
		endLine = len(lines) // Clamp to file end
	}

	// Extract lines (convert from 1-indexed to 0-indexed)
	selectedLines := lines[startLine-1 : endLine]
	return strings.Join(selectedLines, "\n"), nil
}

// ApplyExpansions applies expansion directives to a reference
func (e *ExpansionEngine) ApplyExpansions(
	ctx context.Context,
	ref types.ContextRef,
	hydratedRef *types.HydratedRef,
	format types.FormatType,
	remainingTokens int,
	projectRoot string,
) (int, error) {
	if len(ref.X) == 0 {
		return 0, nil // No expansions
	}

	totalTokens := 0
	visited := make(map[string]struct{}) // Cycle detection

	for _, directive := range ref.X {
		if totalTokens >= remainingTokens {
			break // Token budget exceeded
		}

		directiveType, depth := ParseExpansionDirective(directive)

		var expandedRefs []types.HydratedRef
		var err error

		switch directiveType {
		case "callers":
			expandedRefs, err = e.expandCallers(ctx, ref, depth, visited, remainingTokens-totalTokens, projectRoot, format)
		case "callees":
			expandedRefs, err = e.expandCallees(ctx, ref, depth, visited, remainingTokens-totalTokens, projectRoot, format)
		case "implementations":
			expandedRefs, err = e.expandImplementations(ctx, ref, remainingTokens-totalTokens, projectRoot, format)
		case "interface":
			expandedRefs, err = e.expandInterface(ctx, ref, remainingTokens-totalTokens, projectRoot, format)
		case "siblings":
			expandedRefs, err = e.expandSiblings(ctx, ref, remainingTokens-totalTokens, projectRoot, format)
		case "type_deps":
			expandedRefs, err = e.expandTypeDeps(ctx, ref, remainingTokens-totalTokens, projectRoot, format)
		case "tests":
			expandedRefs, err = e.expandTests(ctx, ref, remainingTokens-totalTokens, projectRoot, format)
		case "doc":
			// Extract just documentation
			err = e.extractDocumentation(hydratedRef, ref)
		case "signature":
			// Extract just signature
			e.extractSignatureOnly(hydratedRef)
		default:
			// Unknown directive, skip with warning
			continue
		}

		if err != nil {
			return totalTokens, fmt.Errorf("failed to expand %s: %w", directive, err)
		}

		// Store expanded refs
		if len(expandedRefs) > 0 {
			hydratedRef.Expanded[directiveType] = expandedRefs

			// Count tokens
			for _, expandedRef := range expandedRefs {
				totalTokens += len(expandedRef.Source) / 4
			}
		}
	}

	return totalTokens, nil
}

// expandCallers expands to functions that call this symbol
func (e *ExpansionEngine) expandCallers(
	ctx context.Context,
	ref types.ContextRef,
	depth int,
	visited map[string]struct{},
	remainingTokens int,
	projectRoot string,
	format types.FormatType,
) ([]types.HydratedRef, error) {
	if e.refTracker == nil {
		return nil, nil
	}

	// Need to get the symbol ID first - look up by file+symbol name
	if ref.S == "" {
		return nil, nil // Can't expand callers without symbol name
	}

	// Find symbol ID by name (simplified - in production would use file context)
	symbols := e.refTracker.FindSymbolsByName(ref.S)
	if len(symbols) == 0 {
		return nil, nil
	}

	// Use first matching symbol (TODO: filter by file for better accuracy)
	symbolID := symbols[0].ID

	// Get callers at current depth - fetch first for capacity calculation
	callerIDs := e.refTracker.GetCallerSymbols(symbolID)
	if len(callerIDs) == 0 {
		return nil, nil
	}

	// Pre-allocate with capacity hint
	results := make([]types.HydratedRef, 0, len(callerIDs))
	totalTokens := 0

	for _, callerID := range callerIDs {
		if totalTokens >= remainingTokens {
			break // Token budget exceeded
		}

		// Get enhanced symbol information
		callerSymbol := e.refTracker.GetEnhancedSymbol(callerID)
		if callerSymbol == nil {
			continue
		}

		// Check if we've already visited this symbol (cycle detection)
		visitKey := fmt.Sprintf("%d", callerID)
		if _, seen := visited[visitKey]; seen {
			continue
		}
		visited[visitKey] = struct{}{}

		// Get file path for the caller symbol
		var callerFilePath string
		if e.index != nil {
			// Try to get file path from MasterIndex
			if masterIndex, ok := e.index.(interface {
				GetFileInfo(types.FileID) *types.FileInfo
			}); ok {
				if fileInfo := masterIndex.GetFileInfo(callerSymbol.FileID); fileInfo != nil {
					callerFilePath = fileInfo.Path
				}
			}
		}
		if callerFilePath == "" {
			continue // Can't resolve file path, skip
		}

		// Create reference for caller
		callerRef := types.ContextRef{
			F: callerFilePath,
			S: callerSymbol.Name,
			L: &types.LineRange{
				Start: callerSymbol.Line,
				End:   callerSymbol.EndLine,
			},
		}

		// Hydrate the caller
		hydratedCaller, tokens, err := e.HydrateReference(ctx, callerRef, format, projectRoot)
		if err != nil {
			continue // Skip on error
		}

		totalTokens += tokens
		results = append(results, *hydratedCaller)

		// Recurse if depth > 1
		if depth > 1 && totalTokens < remainingTokens {
			nestedCallers, err := e.expandCallers(ctx, callerRef, depth-1, visited, remainingTokens-totalTokens, projectRoot, format)
			if err == nil && len(nestedCallers) > 0 {
				// Store nested results in the expanded map
				if hydratedCaller.Expanded == nil {
					hydratedCaller.Expanded = make(map[string][]types.HydratedRef)
				}
				hydratedCaller.Expanded["callers"] = nestedCallers
				for _, nested := range nestedCallers {
					totalTokens += len(nested.Source) / 4
				}
			}
		}
	}

	return results, nil
}

// expandCallees expands to functions that this symbol calls
// Returns internal callees with purity info, and external dependencies separately
func (e *ExpansionEngine) expandCallees(
	ctx context.Context,
	ref types.ContextRef,
	depth int,
	visited map[string]struct{},
	remainingTokens int,
	projectRoot string,
	format types.FormatType,
) ([]types.HydratedRef, error) {
	if e.refTracker == nil {
		return nil, nil
	}

	// Need to get the symbol ID first - look up by file+symbol name
	if ref.S == "" {
		return nil, nil // Can't expand callees without symbol name
	}

	// Find symbol ID by name (simplified - in production would use file context)
	symbols := e.refTracker.FindSymbolsByName(ref.S)
	if len(symbols) == 0 {
		return nil, nil
	}

	// Use first matching symbol (TODO: filter by file for better accuracy)
	symbolID := symbols[0].ID

	// Get callees at current depth - fetch first for capacity calculation
	calleeIDs := e.refTracker.GetCalleeSymbols(symbolID)

	// Get side effect info for the parent symbol to find external dependencies
	parentSideEffects := e.getSideEffectInfo(symbolID)

	// Calculate capacity hint
	externalCount := 0
	if parentSideEffects != nil {
		externalCount = len(parentSideEffects.ExternalCalls)
	}

	if len(calleeIDs) == 0 && externalCount == 0 {
		return nil, nil
	}

	// Pre-allocate with capacity hint (internal + external)
	results := make([]types.HydratedRef, 0, len(calleeIDs)+externalCount)
	totalTokens := 0

	// Process internal callees (symbols in our codebase)
	for _, calleeID := range calleeIDs {
		if totalTokens >= remainingTokens {
			break // Token budget exceeded
		}

		// Get enhanced symbol information
		calleeSymbol := e.refTracker.GetEnhancedSymbol(calleeID)
		if calleeSymbol == nil {
			continue
		}

		// Check if we've already visited this symbol (cycle detection)
		visitKey := fmt.Sprintf("%d", calleeID)
		if _, seen := visited[visitKey]; seen {
			continue
		}
		visited[visitKey] = struct{}{}

		// Get file path for the callee symbol
		var calleeFilePath string
		if e.index != nil {
			// Try to get file path from MasterIndex
			if masterIndex, ok := e.index.(interface {
				GetFileInfo(types.FileID) *types.FileInfo
			}); ok {
				if fileInfo := masterIndex.GetFileInfo(calleeSymbol.FileID); fileInfo != nil {
					calleeFilePath = fileInfo.Path
				}
			}
		}
		if calleeFilePath == "" {
			continue // Can't resolve file path, skip
		}

		// Create reference for callee
		calleeRef := types.ContextRef{
			F: calleeFilePath,
			S: calleeSymbol.Name,
			L: &types.LineRange{
				Start: calleeSymbol.Line,
				End:   calleeSymbol.EndLine,
			},
		}

		// Hydrate the callee
		hydratedCallee, tokens, err := e.HydrateReference(ctx, calleeRef, format, projectRoot)
		if err != nil {
			continue // Skip on error
		}

		// Add purity info for internal callee
		hydratedCallee.Purity = e.getPurityInfo(calleeID)
		hydratedCallee.IsExternal = false

		totalTokens += tokens
		results = append(results, *hydratedCallee)

		// Recurse if depth > 1
		if depth > 1 && totalTokens < remainingTokens {
			nestedCallees, err := e.expandCallees(ctx, calleeRef, depth-1, visited, remainingTokens-totalTokens, projectRoot, format)
			if err == nil && len(nestedCallees) > 0 {
				// Store nested results in the expanded map
				if hydratedCallee.Expanded == nil {
					hydratedCallee.Expanded = make(map[string][]types.HydratedRef)
				}
				hydratedCallee.Expanded["callees"] = nestedCallees
				for _, nested := range nestedCallees {
					totalTokens += len(nested.Source) / 4
				}
			}
		}
	}

	// Add external dependencies (calls to functions outside our codebase)
	if parentSideEffects != nil && len(parentSideEffects.ExternalCalls) > 0 {
		for _, extCall := range parentSideEffects.ExternalCalls {
			if totalTokens >= remainingTokens {
				break
			}

			// Create a minimal HydratedRef for external dependency
			externalRef := types.HydratedRef{
				Symbol:     extCall.FunctionName,
				IsExternal: true,
				Lines:      types.LineRange{Start: extCall.Line, End: extCall.Line},
			}

			// Set file to package name if available
			if extCall.Package != "" {
				externalRef.File = extCall.Package
			}

			// Create purity info for external call
			// External calls are assumed impure unless we have specific knowledge
			externalRef.Purity = &types.PurityInfo{
				IsPure:      false,
				PurityLevel: "ExternalDependency",
				Categories:  []string{"external_call"},
			}

			// Add reason if available
			if extCall.Reason != "" {
				externalRef.Purity.Reasons = []string{extCall.Reason}
			}

			// Add signature hint from the call
			if extCall.IsMethod && extCall.ReceiverType != "" {
				externalRef.Signature = fmt.Sprintf("(%s).%s", extCall.ReceiverType, extCall.FunctionName)
			} else if extCall.Package != "" {
				externalRef.Signature = fmt.Sprintf("%s.%s", extCall.Package, extCall.FunctionName)
			} else {
				externalRef.Signature = extCall.FunctionName
			}

			results = append(results, externalRef)
		}
	}

	return results, nil
}

// expandImplementations finds types implementing an interface.
// Given a symbol reference to an interface, returns all types that implement it.
// Also returns types that extend a base type (for class hierarchies).
func (e *ExpansionEngine) expandImplementations(
	ctx context.Context,
	ref types.ContextRef,
	remainingTokens int,
	projectRoot string,
	format types.FormatType,
) ([]types.HydratedRef, error) {
	if e.refTracker == nil {
		return nil, nil // Use nil instead of empty slice
	}

	if ref.S == "" {
		return nil, nil // Use nil instead of empty slice
	}

	// Find the interface/base type symbol
	symbols := e.refTracker.FindSymbolsByName(ref.S)
	if len(symbols) == 0 {
		return nil, nil // Use nil instead of empty slice
	}

	// Filter to find the appropriate symbol (prefer interface/type definitions)
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		// Prefer interfaces for "implementations" expansion
		if sym.Type == types.SymbolTypeInterface {
			targetSymbol = sym
			break
		}
		// Also support class/struct for derived types
		if sym.Type == types.SymbolTypeClass || sym.Type == types.SymbolTypeStruct || sym.Type == types.SymbolTypeType {
			if targetSymbol == nil {
				targetSymbol = sym
			}
		}
	}

	if targetSymbol == nil {
		// Fallback to first symbol
		targetSymbol = symbols[0]
	}

	// Get implementors (sorted by quality - highest confidence first) and derived types
	implementorsWithQuality := e.refTracker.GetImplementorsWithQuality(targetSymbol.ID)
	derivedIDs := e.refTracker.GetDerivedTypes(targetSymbol.ID)

	// Pre-allocate with combined capacity hint
	capHint := len(implementorsWithQuality) + len(derivedIDs)
	if capHint == 0 {
		return nil, nil
	}
	results := make([]types.HydratedRef, 0, capHint)
	totalTokens := 0
	seen := make(map[types.SymbolID]struct{}, capHint)

	// Process implementors in quality order (highest confidence first)
	for _, impl := range implementorsWithQuality {
		if totalTokens >= remainingTokens {
			break
		}
		if _, ok := seen[impl.SymbolID]; ok {
			continue
		}
		seen[impl.SymbolID] = struct{}{}
		implID := impl.SymbolID

		implSymbol := e.refTracker.GetEnhancedSymbol(implID)
		if implSymbol == nil {
			continue
		}

		filePath := e.getFilePath(implSymbol.FileID)
		if filePath == "" {
			continue
		}

		implRef := types.ContextRef{
			F: filePath,
			S: implSymbol.Name,
			L: &types.LineRange{
				Start: implSymbol.Line,
				End:   implSymbol.EndLine,
			},
		}

		hydratedImpl, tokens, err := e.HydrateReference(ctx, implRef, format, projectRoot)
		if err != nil {
			continue
		}

		totalTokens += tokens
		results = append(results, *hydratedImpl)
	}

	// Process derived types (types that extend this base type) - already fetched above
	for _, derivedID := range derivedIDs {
		if totalTokens >= remainingTokens {
			break
		}
		if _, ok := seen[derivedID]; ok {
			continue
		}
		seen[derivedID] = struct{}{}

		derivedSymbol := e.refTracker.GetEnhancedSymbol(derivedID)
		if derivedSymbol == nil {
			continue
		}

		filePath := e.getFilePath(derivedSymbol.FileID)
		if filePath == "" {
			continue
		}

		derivedRef := types.ContextRef{
			F: filePath,
			S: derivedSymbol.Name,
			L: &types.LineRange{
				Start: derivedSymbol.Line,
				End:   derivedSymbol.EndLine,
			},
		}

		hydratedDerived, tokens, err := e.HydrateReference(ctx, derivedRef, format, projectRoot)
		if err != nil {
			continue
		}

		totalTokens += tokens
		results = append(results, *hydratedDerived)
	}

	return results, nil
}

// expandInterface finds interfaces that a type implements.
// Given a symbol reference to a type, returns all interfaces it implements
// and all base types it extends (for inheritance chains).
func (e *ExpansionEngine) expandInterface(
	ctx context.Context,
	ref types.ContextRef,
	remainingTokens int,
	projectRoot string,
	format types.FormatType,
) ([]types.HydratedRef, error) {
	if e.refTracker == nil {
		return nil, nil
	}

	if ref.S == "" {
		return nil, nil // Need symbol name
	}

	// Find the type symbol
	symbols := e.refTracker.FindSymbolsByName(ref.S)
	if len(symbols) == 0 {
		return nil, nil
	}

	// Filter to find the appropriate symbol (prefer class/struct/type definitions)
	var targetSymbol *types.EnhancedSymbol
	for _, sym := range symbols {
		if sym.Type == types.SymbolTypeClass || sym.Type == types.SymbolTypeStruct || sym.Type == types.SymbolTypeType {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		// Fallback to first symbol
		targetSymbol = symbols[0]
	}

	// Pre-fetch interfaces (sorted by quality) and base types for capacity calculation
	interfacesWithQuality := e.refTracker.GetImplementedInterfacesWithQuality(targetSymbol.ID)
	baseIDs := e.refTracker.GetBaseTypes(targetSymbol.ID)

	// Pre-allocate with combined capacity hint
	capHint := len(interfacesWithQuality) + len(baseIDs)
	if capHint == 0 {
		return nil, nil
	}
	results := make([]types.HydratedRef, 0, capHint)
	seen := make(map[types.SymbolID]struct{}, capHint)
	totalTokens := 0

	// Get interfaces this type implements (sorted by quality - highest confidence first)
	for _, iface := range interfacesWithQuality {
		if totalTokens >= remainingTokens {
			break
		}
		ifaceID := iface.SymbolID
		if _, ok := seen[ifaceID]; ok {
			continue
		}
		seen[ifaceID] = struct{}{}

		ifaceSymbol := e.refTracker.GetEnhancedSymbol(ifaceID)
		if ifaceSymbol == nil {
			continue
		}

		filePath := e.getFilePath(ifaceSymbol.FileID)
		if filePath == "" {
			continue
		}

		ifaceRef := types.ContextRef{
			F: filePath,
			S: ifaceSymbol.Name,
			L: &types.LineRange{
				Start: ifaceSymbol.Line,
				End:   ifaceSymbol.EndLine,
			},
		}

		hydratedIface, tokens, err := e.HydrateReference(ctx, ifaceRef, format, projectRoot)
		if err != nil {
			continue
		}

		totalTokens += tokens
		results = append(results, *hydratedIface)
	}

	// Also get base types (types this type extends) - already fetched above
	for _, baseID := range baseIDs {
		if totalTokens >= remainingTokens {
			break
		}
		if _, ok := seen[baseID]; ok {
			continue
		}
		seen[baseID] = struct{}{}

		baseSymbol := e.refTracker.GetEnhancedSymbol(baseID)
		if baseSymbol == nil {
			continue
		}

		filePath := e.getFilePath(baseSymbol.FileID)
		if filePath == "" {
			continue
		}

		baseRef := types.ContextRef{
			F: filePath,
			S: baseSymbol.Name,
			L: &types.LineRange{
				Start: baseSymbol.Line,
				End:   baseSymbol.EndLine,
			},
		}

		hydratedBase, tokens, err := e.HydrateReference(ctx, baseRef, format, projectRoot)
		if err != nil {
			continue
		}

		totalTokens += tokens
		results = append(results, *hydratedBase)
	}

	return results, nil
}

// expandSiblings finds other methods on the same type
func (e *ExpansionEngine) expandSiblings(
	ctx context.Context,
	ref types.ContextRef,
	remainingTokens int,
	projectRoot string,
	format types.FormatType,
) ([]types.HydratedRef, error) {
	if e.refTracker == nil {
		return nil, nil
	}

	// Need symbol name to find siblings
	if ref.S == "" {
		return nil, nil
	}

	// Find the symbol
	symbols := e.refTracker.FindSymbolsByName(ref.S)
	if len(symbols) == 0 {
		return nil, nil
	}

	symbol := symbols[0]

	// Only methods can have siblings (other methods on the same receiver)
	if symbol.Type != types.SymbolTypeMethod {
		return nil, nil
	}

	// Get the receiver type - this is what we'll use to find siblings
	receiverType := symbol.ReceiverType

	// Get all symbols in the same file
	fileSymbols := e.refTracker.GetFileEnhancedSymbols(symbol.FileID)
	if len(fileSymbols) == 0 {
		return nil, nil
	}

	// Pre-allocate with reasonable capacity - typically 5-10 methods per type
	results := make([]types.HydratedRef, 0, 8)
	totalTokens := 0

	// Find other methods with the same receiver type
	// If ReceiverType is empty (parser doesn't set it), fall back to including
	// all methods in the same file as potential siblings
	for _, sibling := range fileSymbols {
		if totalTokens >= remainingTokens {
			break
		}

		// Skip self
		if sibling.ID == symbol.ID {
			continue
		}

		// Only include methods
		if sibling.Type != types.SymbolTypeMethod {
			continue
		}

		// Check if receiver type matches (if available)
		// When ReceiverType is empty, include all methods in same file as fallback
		if receiverType != "" && sibling.ReceiverType != receiverType {
			continue
		}

		// Get file path from FileID
		var siblingFilePath string
		if e.index != nil {
			if masterIndex, ok := e.index.(interface {
				GetFileInfo(types.FileID) *types.FileInfo
			}); ok {
				if fileInfo := masterIndex.GetFileInfo(sibling.FileID); fileInfo != nil {
					siblingFilePath = fileInfo.Path
				}
			}
		}
		if siblingFilePath == "" {
			continue
		}

		siblingRef := types.ContextRef{
			F: siblingFilePath,
			S: sibling.Name,
			L: &types.LineRange{
				Start: sibling.Line,
				End:   sibling.EndLine,
			},
		}

		// Hydrate the sibling
		hydratedSibling, tokens, err := e.HydrateReference(ctx, siblingRef, format, projectRoot)
		if err != nil {
			continue
		}

		totalTokens += tokens
		results = append(results, *hydratedSibling)
	}

	return results, nil
}

// expandTypeDeps finds types referenced in a function's signature (parameters and return types)
func (e *ExpansionEngine) expandTypeDeps(
	ctx context.Context,
	ref types.ContextRef,
	remainingTokens int,
	projectRoot string,
	format types.FormatType,
) ([]types.HydratedRef, error) {
	if e.refTracker == nil {
		return nil, nil
	}

	if ref.S == "" {
		return nil, nil
	}

	// Find the symbol
	symbols := e.refTracker.FindSymbolsByName(ref.S)
	if len(symbols) == 0 {
		return nil, nil
	}

	symbol := symbols[0]

	// Get the symbol's signature (if available)
	signature := symbol.Signature
	if signature == "" {
		// Try to get from source
		fileInfo := e.getFileInfo(symbol.FileID)
		if fileInfo == nil {
			return nil, nil
		}

		source, err := e.extractSourceByLines(fileInfo.Path, symbol.Line, symbol.Line)
		if err != nil {
			return nil, nil
		}
		signature = strings.TrimSpace(source)
	}

	// Extract type names from signature using simple heuristics
	typeNames := e.extractTypeNamesFromSignature(signature)
	if len(typeNames) == 0 {
		return nil, nil
	}

	// Pre-allocate with capacity based on extracted type names
	results := make([]types.HydratedRef, 0, len(typeNames))
	seen := make(map[string]struct{}, len(typeNames))
	totalTokens := 0

	for _, typeName := range typeNames {
		if totalTokens >= remainingTokens {
			break
		}

		// Skip if already seen
		if _, ok := seen[typeName]; ok {
			continue
		}
		seen[typeName] = struct{}{}

		// Skip built-in types
		if isBuiltinType(typeName) {
			continue
		}

		// Look up the type in the index
		typeSymbols := e.refTracker.FindSymbolsByName(typeName)
		if len(typeSymbols) == 0 {
			continue
		}

		// Find a type definition (struct, interface, type alias)
		var typeSymbol *types.EnhancedSymbol
		for _, sym := range typeSymbols {
			if sym.Type == types.SymbolTypeStruct ||
				sym.Type == types.SymbolTypeInterface ||
				sym.Type == types.SymbolTypeType ||
				sym.Type == types.SymbolTypeClass {
				typeSymbol = sym
				break
			}
		}

		if typeSymbol == nil {
			continue
		}

		// Get file path
		typeFilePath := e.getFilePath(typeSymbol.FileID)
		if typeFilePath == "" {
			continue
		}

		typeRef := types.ContextRef{
			F: typeFilePath,
			S: typeSymbol.Name,
			L: &types.LineRange{
				Start: typeSymbol.Line,
				End:   typeSymbol.EndLine,
			},
		}

		hydratedType, tokens, err := e.HydrateReference(ctx, typeRef, format, projectRoot)
		if err != nil {
			continue
		}

		totalTokens += tokens
		results = append(results, *hydratedType)
	}

	return results, nil
}

// extractTypeNamesFromSignature extracts type names from a Go function signature
func (e *ExpansionEngine) extractTypeNamesFromSignature(signature string) []string {
	var typeNames []string

	// Remove "func " prefix and function name
	signature = strings.TrimPrefix(signature, "func ")

	// Handle method receiver: (r *Type)
	if strings.HasPrefix(signature, "(") {
		endReceiver := strings.Index(signature, ")")
		if endReceiver > 0 {
			receiver := signature[1:endReceiver]
			// Extract type from receiver (e.g., "*Calculator" -> "Calculator")
			parts := strings.Fields(receiver)
			if len(parts) >= 2 {
				typeName := strings.TrimPrefix(parts[1], "*")
				typeNames = append(typeNames, typeName)
			}
			signature = strings.TrimSpace(signature[endReceiver+1:])
		}
	}

	// Skip function name
	funcNameEnd := strings.Index(signature, "(")
	if funcNameEnd > 0 {
		signature = signature[funcNameEnd:]
	}

	// Find parameter list: (params...)
	if strings.HasPrefix(signature, "(") {
		endParams := findMatchingParen(signature, 0)
		if endParams > 0 {
			params := signature[1:endParams]
			typeNames = append(typeNames, extractTypesFromParamList(params)...)
			signature = strings.TrimSpace(signature[endParams+1:])
		}
	}

	// Find return types: either single type or (types...)
	if len(signature) > 0 {
		signature = strings.TrimSpace(signature)
		if strings.HasPrefix(signature, "(") {
			// Multiple return types
			endReturns := findMatchingParen(signature, 0)
			if endReturns > 0 {
				returns := signature[1:endReturns]
				typeNames = append(typeNames, extractTypesFromParamList(returns)...)
			}
		} else if signature != "{" && !strings.HasPrefix(signature, "{") {
			// Single return type
			returnType := strings.TrimSuffix(strings.TrimSpace(signature), "{")
			returnType = strings.TrimSpace(returnType)
			if returnType != "" {
				typeNames = append(typeNames, extractBaseType(returnType))
			}
		}
	}

	return typeNames
}

// findMatchingParen finds the index of the matching closing parenthesis
func findMatchingParen(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '(' {
			depth++
		} else if s[i] == ')' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractTypesFromParamList extracts type names from a parameter list
func extractTypesFromParamList(params string) []string {
	var types []string

	// Split by comma, but be careful of nested types like func(int, int)
	parts := splitParams(params)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle "name type" or just "type"
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}

		// Last field is typically the type
		typeName := fields[len(fields)-1]
		types = append(types, extractBaseType(typeName))
	}

	return types
}

// splitParams splits parameters by comma, respecting nested parentheses
func splitParams(s string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, c := range s {
		if c == '(' || c == '[' || c == '{' {
			depth++
			current.WriteRune(c)
		} else if c == ')' || c == ']' || c == '}' {
			depth--
			current.WriteRune(c)
		} else if c == ',' && depth == 0 {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteRune(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// extractBaseType extracts the base type name from a type expression
func extractBaseType(typeName string) string {
	// Remove pointer prefix
	typeName = strings.TrimPrefix(typeName, "*")
	// Remove slice prefix
	typeName = strings.TrimPrefix(typeName, "[]")
	// Remove map prefix
	if strings.HasPrefix(typeName, "map[") {
		// For map, we'd need to extract both key and value types
		// For now, just return empty
		return ""
	}
	// Remove channel prefix
	typeName = strings.TrimPrefix(typeName, "chan ")
	typeName = strings.TrimPrefix(typeName, "<-chan ")
	typeName = strings.TrimPrefix(typeName, "chan<- ")
	// Remove interface{} and similar
	if typeName == "interface{}" || typeName == "any" {
		return ""
	}
	// Remove variadic prefix
	typeName = strings.TrimPrefix(typeName, "...")

	// Remove package prefix (e.g., "context.Context" -> "Context")
	if idx := strings.LastIndex(typeName, "."); idx >= 0 {
		typeName = typeName[idx+1:]
	}

	return strings.TrimSpace(typeName)
}

// isBuiltinType checks if a type name is a Go built-in type
func isBuiltinType(name string) bool {
	builtins := map[string]bool{
		"bool": true, "byte": true, "complex64": true, "complex128": true,
		"error": true, "float32": true, "float64": true,
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"rune": true, "string": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"uintptr": true, "any": true,
	}
	return builtins[name]
}

// getFileInfo gets file info from the index
func (e *ExpansionEngine) getFileInfo(fileID types.FileID) *types.FileInfo {
	if e.index == nil {
		return nil
	}
	if masterIndex, ok := e.index.(interface {
		GetFileInfo(types.FileID) *types.FileInfo
	}); ok {
		return masterIndex.GetFileInfo(fileID)
	}
	return nil
}

// getFilePath gets file path from file ID
func (e *ExpansionEngine) getFilePath(fileID types.FileID) string {
	fileInfo := e.getFileInfo(fileID)
	if fileInfo != nil {
		return fileInfo.Path
	}
	return ""
}

// expandTests finds test functions for a symbol using multiple discovery strategies:
// 1. Exact match: Test{SymbolName}
// 2. Prefix match: Test{SymbolName}_*
// 3. Reference-based: Tests that call the symbol
func (e *ExpansionEngine) expandTests(
	ctx context.Context,
	ref types.ContextRef,
	remainingTokens int,
	projectRoot string,
	format types.FormatType,
) ([]types.HydratedRef, error) {
	if ref.S == "" {
		return nil, nil // Need symbol name for test discovery
	}

	if e.refTracker == nil {
		return nil, nil
	}

	// Strategy 1: Find test functions with exact name match (Test{SymbolName})
	testFuncName := "Test" + ref.S
	testSymbols := e.refTracker.FindSymbolsByName(testFuncName)

	// Pre-allocate with reasonable capacity - typically 1-5 test functions per symbol
	capHint := len(testSymbols)
	if capHint < 4 {
		capHint = 4
	}
	results := make([]types.HydratedRef, 0, capHint)
	seen := make(map[types.SymbolID]struct{}, capHint)
	totalTokens := 0

	for _, testSym := range testSymbols {
		if totalTokens >= remainingTokens {
			break
		}
		if _, ok := seen[testSym.ID]; ok {
			continue
		}
		seen[testSym.ID] = struct{}{}

		// Verify it's a function in a test file
		filePath := e.getFilePath(testSym.FileID)
		if filePath == "" || !strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		testRef := types.ContextRef{
			F: filePath,
			S: testSym.Name,
			L: &types.LineRange{
				Start: testSym.Line,
				End:   testSym.EndLine,
			},
		}

		hydratedTest, tokens, err := e.HydrateReference(ctx, testRef, format, projectRoot)
		if err != nil {
			continue
		}

		totalTokens += tokens
		results = append(results, *hydratedTest)
	}

	// Strategy 2: Find test functions that start with Test{SymbolName}_ (subtests)
	// Search through all symbols to find matches
	targetSymbols := e.refTracker.FindSymbolsByName(ref.S)
	if len(targetSymbols) == 0 {
		return results, nil
	}

	targetSymbol := targetSymbols[0]

	// Get callers of the target symbol - these might be tests
	callerIDs := e.refTracker.GetCallerSymbols(targetSymbol.ID)
	for _, callerID := range callerIDs {
		if totalTokens >= remainingTokens {
			break
		}
		if _, ok := seen[callerID]; ok {
			continue
		}

		callerSymbol := e.refTracker.GetEnhancedSymbol(callerID)
		if callerSymbol == nil {
			continue
		}

		// Check if it's a test function (starts with "Test" and in a _test.go file)
		if !strings.HasPrefix(callerSymbol.Name, "Test") {
			continue
		}

		filePath := e.getFilePath(callerSymbol.FileID)
		if filePath == "" || !strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		seen[callerID] = struct{}{}

		testRef := types.ContextRef{
			F: filePath,
			S: callerSymbol.Name,
			L: &types.LineRange{
				Start: callerSymbol.Line,
				End:   callerSymbol.EndLine,
			},
		}

		hydratedTest, tokens, err := e.HydrateReference(ctx, testRef, format, projectRoot)
		if err != nil {
			continue
		}

		totalTokens += tokens
		results = append(results, *hydratedTest)
	}

	// Strategy 3: Look in corresponding _test.go file for any Test* functions
	// that might test this symbol (fallback for when call graph doesn't capture it)
	if len(results) == 0 && ref.F != "" {
		testFileName := strings.TrimSuffix(filepath.Base(ref.F), filepath.Ext(ref.F)) + "_test" + filepath.Ext(ref.F)
		testFilePath := filepath.Join(filepath.Dir(ref.F), testFileName)
		fullTestPath := filepath.Join(projectRoot, testFilePath)

		// Check if test file exists and try to find Test{SymbolName} function
		if _, err := e.fileService.ReadFile(fullTestPath); err == nil {
			// File exists, try to find the test function
			testRef := types.ContextRef{
				F: testFilePath,
				S: testFuncName,
			}

			hydratedTest, tokens, err := e.HydrateReference(ctx, testRef, format, projectRoot)
			if err == nil && totalTokens+tokens <= remainingTokens {
				results = append(results, *hydratedTest)
			}
		}
	}

	return results, nil
}

// extractDocumentation extracts just documentation from source
func (e *ExpansionEngine) extractDocumentation(hydratedRef *types.HydratedRef, ref types.ContextRef) error {
	lines := strings.Split(hydratedRef.Source, "\n")
	var docLines []string

	// Extract comment lines at the start
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			docLines = append(docLines, line)
		} else if trimmed != "" {
			break // Stop at first non-comment line
		}
	}

	if len(docLines) > 0 {
		hydratedRef.Source = strings.Join(docLines, "\n")
	}

	return nil
}

// extractSignatureOnly extracts just the signature from source
func (e *ExpansionEngine) extractSignatureOnly(hydratedRef *types.HydratedRef) {
	lines := strings.Split(hydratedRef.Source, "\n")

	// Find the first line that looks like a declaration
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// This is likely the signature line
		hydratedRef.Source = trimmed
		hydratedRef.Signature = trimmed
		break
	}
}

// getPurityInfo returns purity information for a symbol ID
func (e *ExpansionEngine) getPurityInfo(symbolID types.SymbolID) *types.PurityInfo {
	if e.sideEffectPropagator == nil {
		return nil
	}

	// Try to get side effect info from the propagator
	propagator, ok := e.sideEffectPropagator.(interface {
		GetSideEffectInfo(types.SymbolID) *types.SideEffectInfo
	})
	if !ok {
		return nil
	}

	info := propagator.GetSideEffectInfo(symbolID)
	if info == nil {
		return nil
	}

	// Convert SideEffectInfo to PurityInfo
	purity := &types.PurityInfo{
		IsPure:      info.IsPure,
		PurityLevel: info.PurityLevel.String(),
		PurityScore: info.PurityScore,
	}

	// Add side effect categories
	purity.Categories = categoriesToStringSlice(info.Categories)

	// Add transitive categories if present
	if info.TransitiveCategories != types.SideEffectNone {
		transitiveCategories := categoriesToStringSlice(info.TransitiveCategories)
		for _, cat := range transitiveCategories {
			// Add with "transitive:" prefix to distinguish
			purity.Categories = append(purity.Categories, "transitive:"+cat)
		}
	}

	// Add reasons for impurity
	if len(info.ImpurityReasons) > 0 {
		purity.Reasons = info.ImpurityReasons
	}

	return purity
}

// getSideEffectInfo returns the full side effect info for a symbol (used for external calls)
func (e *ExpansionEngine) getSideEffectInfo(symbolID types.SymbolID) *types.SideEffectInfo {
	if e.sideEffectPropagator == nil {
		return nil
	}

	propagator, ok := e.sideEffectPropagator.(interface {
		GetSideEffectInfo(types.SymbolID) *types.SideEffectInfo
	})
	if !ok {
		return nil
	}

	return propagator.GetSideEffectInfo(symbolID)
}

// categoriesToStringSlice converts SideEffectCategory bitfield to string slice
func categoriesToStringSlice(cat types.SideEffectCategory) []string {
	if cat == types.SideEffectNone {
		return nil
	}

	var result []string

	if cat&types.SideEffectParamWrite != 0 {
		result = append(result, "param_write")
	}
	if cat&types.SideEffectReceiverWrite != 0 {
		result = append(result, "receiver_write")
	}
	if cat&types.SideEffectGlobalWrite != 0 {
		result = append(result, "global_write")
	}
	if cat&types.SideEffectClosureWrite != 0 {
		result = append(result, "closure_write")
	}
	if cat&types.SideEffectFieldWrite != 0 {
		result = append(result, "field_write")
	}
	if cat&types.SideEffectIO != 0 {
		result = append(result, "io")
	}
	if cat&types.SideEffectDatabase != 0 {
		result = append(result, "database")
	}
	if cat&types.SideEffectNetwork != 0 {
		result = append(result, "network")
	}
	if cat&types.SideEffectThrow != 0 {
		result = append(result, "throw")
	}
	if cat&types.SideEffectChannel != 0 {
		result = append(result, "channel")
	}
	if cat&types.SideEffectAsync != 0 {
		result = append(result, "async")
	}
	if cat&types.SideEffectExternalCall != 0 {
		result = append(result, "external_call")
	}
	if cat&types.SideEffectDynamicCall != 0 {
		result = append(result, "dynamic_call")
	}
	if cat&types.SideEffectReflection != 0 {
		result = append(result, "reflection")
	}

	return result
}
