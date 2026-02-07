package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/idcodec"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ========== Parameter Types ==========

// ListSymbolsParams for the list_symbols tool
type ListSymbolsParams struct {
	Kind          string `json:"kind"`                    // Required: symbol kinds to list (comma-separated)
	File          string `json:"file,omitempty"`           // Glob pattern for file path filter
	Exported      *bool  `json:"exported,omitempty"`       // true=exported, false=unexported, nil=all
	Name          string `json:"name,omitempty"`           // Substring filter on symbol name (case-insensitive)
	Receiver      string `json:"receiver,omitempty"`       // Filter methods by receiver type
	MinComplexity *int   `json:"min_complexity,omitempty"` // Min cyclomatic complexity
	MaxComplexity *int   `json:"max_complexity,omitempty"` // Max cyclomatic complexity
	MinParams     *int   `json:"min_params,omitempty"`     // Min parameter count
	MaxParams     *int   `json:"max_params,omitempty"`     // Max parameter count
	Flags         string `json:"flags,omitempty"`          // Comma-separated: async, variadic, generator, method
	Sort          string `json:"sort,omitempty"`           // name (default), complexity, refs, line, params
	Max           int    `json:"max,omitempty"`            // Max results (default 50, max 500)
	Offset        int    `json:"offset,omitempty"`         // Pagination offset
	Include       string `json:"include,omitempty"`        // Comma-separated extras: signature, doc, refs, callers, callees, scope, ids, all

	Warnings []UnknownField `json:"-"`
}

// UnmarshalJSON for ListSymbolsParams with unknown field tracking
func (p *ListSymbolsParams) UnmarshalJSON(data []byte) error {
	type Alias ListSymbolsParams
	knownFields := map[string]struct{}{
		"kind": {}, "file": {}, "exported": {}, "name": {}, "receiver": {},
		"min_complexity": {}, "max_complexity": {}, "min_params": {}, "max_params": {},
		"flags": {}, "sort": {}, "max": {}, "offset": {}, "include": {},
	}
	_, warnings, err := collectUnknownFields(data, knownFields, nil)
	if err != nil {
		return err
	}
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*p = ListSymbolsParams(alias)
	p.Warnings = warnings
	return nil
}

// InspectSymbolParams for the inspect_symbol tool
type InspectSymbolParams struct {
	Name     string `json:"name,omitempty"`      // Symbol name (exact match)
	ID       string `json:"id,omitempty"`        // Object ID from search/list_symbols
	File     string `json:"file,omitempty"`      // File path pattern to disambiguate
	Type     string `json:"type,omitempty"`      // Symbol type to disambiguate
	Include  string `json:"include,omitempty"`   // Sections: signature, doc, callers, callees, type_hierarchy, scope, refs, annotations, flags, all
	MaxDepth int    `json:"max_depth,omitempty"` // Max depth for hierarchy traversal (default 3)

	Warnings []UnknownField `json:"-"`
}

// UnmarshalJSON for InspectSymbolParams with unknown field tracking
func (p *InspectSymbolParams) UnmarshalJSON(data []byte) error {
	type Alias InspectSymbolParams
	knownFields := map[string]struct{}{
		"name": {}, "id": {}, "file": {}, "type": {}, "include": {}, "max_depth": {},
	}
	_, warnings, err := collectUnknownFields(data, knownFields, nil)
	if err != nil {
		return err
	}
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*p = InspectSymbolParams(alias)
	p.Warnings = warnings
	return nil
}

// BrowseFileParams for the browse_file tool
type BrowseFileParams struct {
	File        string `json:"file,omitempty"`         // File path (exact or glob)
	FileID      *int   `json:"file_id,omitempty"`      // File ID alternative
	Kind        string `json:"kind,omitempty"`          // Symbol kind filter
	Exported    *bool  `json:"exported,omitempty"`      // Visibility filter
	Sort        string `json:"sort,omitempty"`          // line (default), name, type, complexity, refs
	Max         int    `json:"max,omitempty"`           // Max symbols (default 100)
	Include     string `json:"include,omitempty"`       // Same as list_symbols
	ShowImports bool   `json:"show_imports,omitempty"`  // Include import list
	ShowStats   bool   `json:"show_stats,omitempty"`    // Include file-level stats

	Warnings []UnknownField `json:"-"`
}

// UnmarshalJSON for BrowseFileParams with unknown field tracking
func (p *BrowseFileParams) UnmarshalJSON(data []byte) error {
	type Alias BrowseFileParams
	knownFields := map[string]struct{}{
		"file": {}, "file_id": {}, "kind": {}, "exported": {},
		"sort": {}, "max": {}, "include": {}, "show_imports": {}, "show_stats": {},
	}
	_, warnings, err := collectUnknownFields(data, knownFields, nil)
	if err != nil {
		return err
	}
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*p = BrowseFileParams(alias)
	p.Warnings = warnings
	return nil
}

// ========== Response Types ==========

// ExploreSymbol is a symbol entry returned by list_symbols/browse_file
type ExploreSymbol struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	File           string   `json:"file"`
	Line           int      `json:"line"`
	ObjectID       string   `json:"object_id,omitempty"`
	IsExported     bool     `json:"is_exported"`
	Signature      string   `json:"signature,omitempty"`
	DocComment     string   `json:"doc_comment,omitempty"`
	Complexity     int      `json:"complexity,omitempty"`
	ParameterCount int      `json:"parameter_count,omitempty"`
	ReceiverType   string   `json:"receiver_type,omitempty"`
	IncomingRefs   int      `json:"incoming_refs,omitempty"`
	OutgoingRefs   int      `json:"outgoing_refs,omitempty"`
	Callers        []string `json:"callers,omitempty"`
	Callees        []string `json:"callees,omitempty"`
	ScopeChain     []string `json:"scope_chain,omitempty"`
}

// ListSymbolsResponse for list_symbols
type ListSymbolsResponse struct {
	Symbols  []ExploreSymbol `json:"symbols"`
	Total    int             `json:"total"`
	Showing  int             `json:"showing"`
	HasMore  bool            `json:"has_more"`
	Warnings []UnknownField  `json:"warnings,omitempty"`
}

// InspectSymbolResult is a deep-inspect result for a single symbol
type InspectSymbolResult struct {
	Name           string              `json:"name"`
	ObjectID       string              `json:"object_id"`
	Type           string              `json:"type"`
	File           string              `json:"file"`
	Line           int                 `json:"line"`
	IsExported     bool                `json:"is_exported"`
	Signature      string              `json:"signature,omitempty"`
	DocComment     string              `json:"doc_comment,omitempty"`
	Complexity     int                 `json:"complexity,omitempty"`
	ParameterCount int                 `json:"parameter_count,omitempty"`
	ReceiverType   string              `json:"receiver_type,omitempty"`
	FunctionFlags  []string            `json:"function_flags,omitempty"`
	VariableFlags  []string            `json:"variable_flags,omitempty"`
	Callers        []string            `json:"callers,omitempty"`
	Callees        []string            `json:"callees,omitempty"`
	TypeHierarchy  *TypeHierarchyInfo  `json:"type_hierarchy,omitempty"`
	ScopeChain     []string            `json:"scope_chain,omitempty"`
	IncomingRefs   int                 `json:"incoming_refs,omitempty"`
	OutgoingRefs   int                 `json:"outgoing_refs,omitempty"`
	Annotations    []string            `json:"annotations,omitempty"`
}

// TypeHierarchyInfo for inspect_symbol type hierarchy
type TypeHierarchyInfo struct {
	Implements    []string `json:"implements,omitempty"`
	ImplementedBy []string `json:"implemented_by,omitempty"`
	Extends       []string `json:"extends,omitempty"`
	ExtendedBy    []string `json:"extended_by,omitempty"`
}

// InspectSymbolResponse for inspect_symbol
type InspectSymbolResponse struct {
	Symbols  []InspectSymbolResult `json:"symbols"`
	Count    int                   `json:"count"`
	Warnings []UnknownField        `json:"warnings,omitempty"`
}

// BrowseFileResponse for browse_file
type BrowseFileResponse struct {
	File     BrowseFileInfo  `json:"file"`
	Symbols  []ExploreSymbol `json:"symbols"`
	Total    int             `json:"total"`
	Imports  []string        `json:"imports,omitempty"`
	Stats    *FileStats      `json:"stats,omitempty"`
	Warnings []UnknownField  `json:"warnings,omitempty"`
}

// BrowseFileInfo describes the file being browsed
type BrowseFileInfo struct {
	Path     string `json:"path"`
	FileID   int    `json:"file_id"`
	Language string `json:"language,omitempty"`
}

// FileStats for browse_file stats
type FileStats struct {
	SymbolCount    int     `json:"symbol_count"`
	FunctionCount  int     `json:"function_count"`
	TypeCount      int     `json:"type_count"`
	AvgComplexity  float64 `json:"avg_complexity,omitempty"`
	MaxComplexity  int     `json:"max_complexity,omitempty"`
	ExportedCount  int     `json:"exported_count"`
}

// ========== Kind Parsing ==========

// parseSymbolKinds converts a comma-separated kind string to a set of SymbolType values.
// Supports aliases: fn/func->function, var->variable, const->constant, cls->class, iface->interface.
func parseSymbolKinds(kindStr string) map[types.SymbolType]bool {
	if kindStr == "" || kindStr == "all" {
		return nil // nil means all kinds
	}
	kinds := make(map[types.SymbolType]bool)
	for _, k := range parseListHelper(kindStr) {
		k = strings.ToLower(k)
		switch k {
		case "func", "fn", "function":
			kinds[types.SymbolTypeFunction] = true
		case "type":
			kinds[types.SymbolTypeType] = true
		case "struct":
			kinds[types.SymbolTypeStruct] = true
		case "interface", "iface":
			kinds[types.SymbolTypeInterface] = true
		case "method":
			kinds[types.SymbolTypeMethod] = true
		case "class", "cls":
			kinds[types.SymbolTypeClass] = true
		case "enum":
			kinds[types.SymbolTypeEnum] = true
		case "variable", "var":
			kinds[types.SymbolTypeVariable] = true
		case "constant", "const":
			kinds[types.SymbolTypeConstant] = true
		case "field":
			kinds[types.SymbolTypeField] = true
		case "property":
			kinds[types.SymbolTypeProperty] = true
		case "module":
			kinds[types.SymbolTypeModule] = true
		case "namespace":
			kinds[types.SymbolTypeNamespace] = true
		case "constructor":
			kinds[types.SymbolTypeConstructor] = true
		case "trait":
			kinds[types.SymbolTypeTrait] = true
		}
	}
	if len(kinds) == 0 {
		return nil
	}
	return kinds
}

// ========== Include Parsing ==========

func exploreIncludes(include string) map[string]bool {
	if include == "" {
		// Defaults: signature, ids
		return map[string]bool{"signature": true, "ids": true}
	}
	m := make(map[string]bool)
	for _, item := range parseListHelper(include) {
		item = strings.ToLower(item)
		if item == "all" {
			return map[string]bool{
				"signature": true, "doc": true, "refs": true,
				"callers": true, "callees": true, "scope": true, "ids": true,
			}
		}
		m[item] = true
	}
	return m
}

func inspectIncludes(include string) map[string]bool {
	if include == "" || include == "all" {
		return map[string]bool{
			"signature": true, "doc": true, "callers": true, "callees": true,
			"type_hierarchy": true, "scope": true, "refs": true,
			"annotations": true, "flags": true,
		}
	}
	m := make(map[string]bool)
	for _, item := range parseListHelper(include) {
		m[strings.ToLower(item)] = true
	}
	return m
}

// ========== Handlers ==========

// handleListSymbols implements the list_symbols MCP tool
func (s *Server) handleListSymbols(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.recoverFromPanic("list_symbols", func() (*mcp.CallToolResult, error) {
		var params ListSymbolsParams
		if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
			return createSmartErrorResponse("list_symbols", fmt.Errorf("invalid parameters: %w", err), nil)
		}

		if params.Kind == "" {
			return createSmartErrorResponse("list_symbols", fmt.Errorf("'kind' parameter is required (e.g., \"func\", \"type\", \"all\")"), map[string]interface{}{
				"example": `{"kind": "func"}`,
			})
		}

		if available, err := s.checkIndexAvailability(); err != nil {
			return createSmartErrorResponse("list_symbols", err, nil)
		} else if !available {
			return createSmartErrorResponse("list_symbols", fmt.Errorf("index not available"), nil)
		}

		tracker := s.GetRefTracker()
		if tracker == nil {
			return createSmartErrorResponse("list_symbols", fmt.Errorf("reference tracker not available"), nil)
		}

		kinds := parseSymbolKinds(params.Kind)
		includes := exploreIncludes(params.Include)

		maxResults := params.Max
		if maxResults <= 0 {
			maxResults = 50
		}
		if maxResults > 500 {
			maxResults = 500
		}

		// Compile file glob pattern if specified
		var filePattern string
		if params.File != "" {
			filePattern = params.File
		}

		// Collect matching symbols from all files
		allFileIDs := s.goroutineIndex.GetAllFileIDsFiltered()
		var allSymbols []*symbolWithFile
		for _, fileID := range allFileIDs {
			filePath := s.GetFilePath(fileID)
			if filePath == "" {
				continue
			}

			// Apply file filter
			if filePattern != "" {
				matched, _ := filepath.Match(filePattern, filePath)
				if !matched {
					// Try matching just the filename
					matched, _ = filepath.Match(filePattern, filepath.Base(filePath))
				}
				if !matched {
					continue
				}
			}

			symbols := s.goroutineIndex.GetFileEnhancedSymbols(fileID)
			for _, sym := range symbols {
				if matchesListFilters(sym, kinds, params) {
					allSymbols = append(allSymbols, &symbolWithFile{sym: sym, filePath: filePath})
				}
			}
		}

		total := len(allSymbols)

		// Sort
		sortSymbols(allSymbols, params.Sort)

		// Apply offset/limit
		if params.Offset > 0 && params.Offset < len(allSymbols) {
			allSymbols = allSymbols[params.Offset:]
		} else if params.Offset >= len(allSymbols) {
			allSymbols = nil
		}
		if len(allSymbols) > maxResults {
			allSymbols = allSymbols[:maxResults]
		}

		// Build response
		result := make([]ExploreSymbol, len(allSymbols))
		for i, swf := range allSymbols {
			result[i] = buildExploreSymbol(swf.sym, swf.filePath, includes, tracker)
		}

		resp := ListSymbolsResponse{
			Symbols:  result,
			Total:    total,
			Showing:  len(result),
			HasMore:  total > params.Offset+len(result),
			Warnings: params.Warnings,
		}

		return createJSONResponse(resp)
	})
}

// handleInspectSymbol implements the inspect_symbol MCP tool
func (s *Server) handleInspectSymbol(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.recoverFromPanic("inspect_symbol", func() (*mcp.CallToolResult, error) {
		var params InspectSymbolParams
		if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
			return createSmartErrorResponse("inspect_symbol", fmt.Errorf("invalid parameters: %w", err), nil)
		}

		if params.Name == "" && params.ID == "" {
			return createSmartErrorResponse("inspect_symbol", fmt.Errorf("either 'name' or 'id' parameter is required"), map[string]interface{}{
				"example_by_name": `{"name": "handleSearch"}`,
				"example_by_id":   `{"id": "VE"}`,
			})
		}

		if available, err := s.checkIndexAvailability(); err != nil {
			return createSmartErrorResponse("inspect_symbol", err, nil)
		} else if !available {
			return createSmartErrorResponse("inspect_symbol", fmt.Errorf("index not available"), nil)
		}

		tracker := s.GetRefTracker()
		if tracker == nil {
			return createSmartErrorResponse("inspect_symbol", fmt.Errorf("reference tracker not available"), nil)
		}

		includes := inspectIncludes(params.Include)
		maxDepth := params.MaxDepth
		if maxDepth <= 0 {
			maxDepth = 3
		}

		var matched []*types.EnhancedSymbol

		// Lookup by ID
		if params.ID != "" {
			for _, idStr := range parseListHelper(params.ID) {
				symbolID, err := idcodec.DecodeSymbolID(idStr)
				if err != nil {
					continue
				}
				if sym := s.GetSymbolByID(symbolID); sym != nil {
					matched = append(matched, sym)
				}
			}
		}

		// Lookup by name
		if params.Name != "" && len(matched) == 0 {
			matched = s.goroutineIndex.FindSymbolsByName(params.Name)
		}

		// Apply file/type disambiguation
		if params.File != "" || params.Type != "" {
			filtered := matched[:0]
			for _, sym := range matched {
				filePath := s.GetFilePath(sym.FileID)
				if params.File != "" {
					m, _ := filepath.Match(params.File, filePath)
					if !m {
						m, _ = filepath.Match(params.File, filepath.Base(filePath))
					}
					if !m {
						continue
					}
				}
				if params.Type != "" {
					expectedKinds := parseSymbolKinds(params.Type)
					if expectedKinds != nil && !expectedKinds[sym.Symbol.Type] {
						continue
					}
				}
				filtered = append(filtered, sym)
			}
			matched = filtered
		}

		// Build detailed results
		results := make([]InspectSymbolResult, len(matched))
		for i, sym := range matched {
			results[i] = buildInspectResult(sym, s.GetFilePath(sym.FileID), includes, tracker, maxDepth, s)
		}

		resp := InspectSymbolResponse{
			Symbols:  results,
			Count:    len(results),
			Warnings: params.Warnings,
		}

		return createJSONResponse(resp)
	})
}

// handleBrowseFile implements the browse_file MCP tool
func (s *Server) handleBrowseFile(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.recoverFromPanic("browse_file", func() (*mcp.CallToolResult, error) {
		var params BrowseFileParams
		if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
			return createSmartErrorResponse("browse_file", fmt.Errorf("invalid parameters: %w", err), nil)
		}

		if params.File == "" && params.FileID == nil {
			return createSmartErrorResponse("browse_file", fmt.Errorf("either 'file' or 'file_id' parameter is required"), map[string]interface{}{
				"example": `{"file": "internal/mcp/server.go"}`,
			})
		}

		if available, err := s.checkIndexAvailability(); err != nil {
			return createSmartErrorResponse("browse_file", err, nil)
		} else if !available {
			return createSmartErrorResponse("browse_file", fmt.Errorf("index not available"), nil)
		}

		tracker := s.GetRefTracker()
		if tracker == nil {
			return createSmartErrorResponse("browse_file", fmt.Errorf("reference tracker not available"), nil)
		}

		// Find the file
		var targetFileID types.FileID
		var targetFilePath string
		found := false

		if params.FileID != nil {
			targetFileID = types.FileID(*params.FileID)
			targetFilePath = s.GetFilePath(targetFileID)
			if targetFilePath != "" {
				found = true
			}
		}

		if !found && params.File != "" {
			// Search all files for a match
			allFileIDs := s.goroutineIndex.GetAllFileIDsFiltered()
			for _, fid := range allFileIDs {
				fp := s.GetFilePath(fid)
				if fp == "" {
					continue
				}
				// Exact match
				if fp == params.File || filepath.Base(fp) == params.File {
					targetFileID = fid
					targetFilePath = fp
					found = true
					break
				}
				// Suffix match (e.g., "server.go" matches "internal/mcp/server.go")
				if strings.HasSuffix(fp, "/"+params.File) || strings.HasSuffix(fp, "\\"+params.File) {
					targetFileID = fid
					targetFilePath = fp
					found = true
					break
				}
				// Glob match
				if m, _ := filepath.Match(params.File, fp); m {
					targetFileID = fid
					targetFilePath = fp
					found = true
					break
				}
			}
		}

		if !found {
			return createSmartErrorResponse("browse_file", fmt.Errorf("file not found: %s", params.File), map[string]interface{}{
				"hint": "Use find_files to locate the correct path, or provide file_id from search results",
			})
		}

		kinds := parseSymbolKinds(params.Kind)
		includes := exploreIncludes(params.Include)

		maxResults := params.Max
		if maxResults <= 0 {
			maxResults = 100
		}

		// Get symbols for the file
		symbols := s.goroutineIndex.GetFileEnhancedSymbols(targetFileID)
		var filtered []*types.EnhancedSymbol
		for _, sym := range symbols {
			if kinds != nil && !kinds[sym.Symbol.Type] {
				continue
			}
			if params.Exported != nil {
				if *params.Exported && !sym.IsExported {
					continue
				}
				if !*params.Exported && sym.IsExported {
					continue
				}
			}
			filtered = append(filtered, sym)
		}

		total := len(filtered)

		// Sort
		sortEnhancedSymbols(filtered, params.Sort)

		// Limit
		if len(filtered) > maxResults {
			filtered = filtered[:maxResults]
		}

		// Build response symbols
		result := make([]ExploreSymbol, len(filtered))
		for i, sym := range filtered {
			result[i] = buildExploreSymbol(sym, targetFilePath, includes, tracker)
		}

		// Determine language from file extension
		lang := languageFromPath(targetFilePath)

		resp := BrowseFileResponse{
			File: BrowseFileInfo{
				Path:     targetFilePath,
				FileID:   int(targetFileID),
				Language: lang,
			},
			Symbols:  result,
			Total:    total,
			Warnings: params.Warnings,
		}

		// Include imports if requested
		if params.ShowImports {
			fileInfo := s.goroutineIndex.GetFile(targetFileID)
			if fileInfo != nil {
				imports := make([]string, len(fileInfo.Imports))
				for i, imp := range fileInfo.Imports {
					imports[i] = imp.Path
				}
				resp.Imports = imports
			}
		}

		// Include stats if requested
		if params.ShowStats {
			stats := computeFileStats(symbols)
			resp.Stats = stats
		}

		return createJSONResponse(resp)
	})
}

// ========== Helper Types and Functions ==========

type symbolWithFile struct {
	sym      *types.EnhancedSymbol
	filePath string
}

// matchesListFilters checks if a symbol matches the list_symbols filter criteria
func matchesListFilters(sym *types.EnhancedSymbol, kinds map[types.SymbolType]bool, params ListSymbolsParams) bool {
	// Kind filter
	if kinds != nil && !kinds[sym.Symbol.Type] {
		return false
	}

	// Exported filter
	if params.Exported != nil {
		if *params.Exported && !sym.IsExported {
			return false
		}
		if !*params.Exported && sym.IsExported {
			return false
		}
	}

	// Name substring filter (case-insensitive)
	if params.Name != "" {
		if !strings.Contains(strings.ToLower(sym.Symbol.Name), strings.ToLower(params.Name)) {
			return false
		}
	}

	// Receiver filter
	if params.Receiver != "" {
		if !strings.EqualFold(sym.ReceiverType, params.Receiver) {
			return false
		}
	}

	// Complexity filters
	if params.MinComplexity != nil && sym.Complexity < *params.MinComplexity {
		return false
	}
	if params.MaxComplexity != nil && sym.Complexity > *params.MaxComplexity {
		return false
	}

	// Parameter count filters
	if params.MinParams != nil && int(sym.ParameterCount) < *params.MinParams {
		return false
	}
	if params.MaxParams != nil && int(sym.ParameterCount) > *params.MaxParams {
		return false
	}

	// Flag filters
	if params.Flags != "" {
		for _, flag := range parseListHelper(params.Flags) {
			switch strings.ToLower(flag) {
			case "async":
				if sym.FunctionFlags&types.FunctionFlagAsync == 0 {
					return false
				}
			case "variadic":
				if sym.FunctionFlags&types.FunctionFlagVariadic == 0 {
					return false
				}
			case "generator":
				if sym.FunctionFlags&types.FunctionFlagGenerator == 0 {
					return false
				}
			case "method":
				if sym.FunctionFlags&types.FunctionFlagMethod == 0 {
					return false
				}
			}
		}
	}

	return true
}

// sortSymbols sorts symbolWithFile entries by the given sort key
func sortSymbols(symbols []*symbolWithFile, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "complexity":
		sort.Slice(symbols, func(i, j int) bool {
			return symbols[i].sym.Complexity > symbols[j].sym.Complexity
		})
	case "refs":
		sort.Slice(symbols, func(i, j int) bool {
			ri := len(symbols[i].sym.IncomingRefs)
			rj := len(symbols[j].sym.IncomingRefs)
			return ri > rj
		})
	case "line":
		sort.Slice(symbols, func(i, j int) bool {
			if symbols[i].filePath != symbols[j].filePath {
				return symbols[i].filePath < symbols[j].filePath
			}
			return symbols[i].sym.Symbol.Line < symbols[j].sym.Symbol.Line
		})
	case "params":
		sort.Slice(symbols, func(i, j int) bool {
			return symbols[i].sym.ParameterCount > symbols[j].sym.ParameterCount
		})
	default: // "name"
		sort.Slice(symbols, func(i, j int) bool {
			return symbols[i].sym.Symbol.Name < symbols[j].sym.Symbol.Name
		})
	}
}

// sortEnhancedSymbols sorts EnhancedSymbol entries by the given sort key
func sortEnhancedSymbols(symbols []*types.EnhancedSymbol, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "name":
		sort.Slice(symbols, func(i, j int) bool {
			return symbols[i].Symbol.Name < symbols[j].Symbol.Name
		})
	case "type":
		sort.Slice(symbols, func(i, j int) bool {
			return symbols[i].Symbol.Type.String() < symbols[j].Symbol.Type.String()
		})
	case "complexity":
		sort.Slice(symbols, func(i, j int) bool {
			return symbols[i].Complexity > symbols[j].Complexity
		})
	case "refs":
		sort.Slice(symbols, func(i, j int) bool {
			return len(symbols[i].IncomingRefs) > len(symbols[j].IncomingRefs)
		})
	default: // "line"
		sort.Slice(symbols, func(i, j int) bool {
			return symbols[i].Symbol.Line < symbols[j].Symbol.Line
		})
	}
}

// buildExploreSymbol converts an EnhancedSymbol to the response format
func buildExploreSymbol(sym *types.EnhancedSymbol, filePath string, includes map[string]bool, tracker *core.ReferenceTracker) ExploreSymbol {
	es := ExploreSymbol{
		Name:       sym.Symbol.Name,
		Type:       sym.Symbol.Type.String(),
		File:       filePath,
		Line:       sym.Symbol.Line,
		IsExported: sym.IsExported,
	}

	if includes["ids"] {
		es.ObjectID = searchtypes.EncodeSymbolID(sym.ID)
	}

	if includes["signature"] && sym.Signature != "" {
		es.Signature = sym.Signature
	}

	if includes["doc"] && sym.DocComment != "" {
		es.DocComment = sym.DocComment
	}

	// Always include complexity for functions/methods if non-zero
	if sym.Complexity > 0 {
		es.Complexity = sym.Complexity
	}
	if sym.ParameterCount > 0 {
		es.ParameterCount = int(sym.ParameterCount)
	}
	if sym.ReceiverType != "" {
		es.ReceiverType = sym.ReceiverType
	}

	if includes["refs"] {
		es.IncomingRefs = len(sym.IncomingRefs)
		es.OutgoingRefs = len(sym.OutgoingRefs)
	}

	if includes["callers"] && tracker != nil {
		es.Callers = tracker.GetCallerNames(sym.ID)
	}

	if includes["callees"] && tracker != nil {
		es.Callees = tracker.GetCalleeNames(sym.ID)
	}

	if includes["scope"] && len(sym.ScopeChain) > 0 {
		chain := make([]string, len(sym.ScopeChain))
		for i, sc := range sym.ScopeChain {
			chain[i] = sc.Name
		}
		es.ScopeChain = chain
	}

	return es
}

// buildInspectResult converts an EnhancedSymbol to a detailed inspect result
func buildInspectResult(sym *types.EnhancedSymbol, filePath string, includes map[string]bool, tracker *core.ReferenceTracker, maxDepth int, s *Server) InspectSymbolResult {
	r := InspectSymbolResult{
		Name:       sym.Symbol.Name,
		ObjectID:   searchtypes.EncodeSymbolID(sym.ID),
		Type:       sym.Symbol.Type.String(),
		File:       filePath,
		Line:       sym.Symbol.Line,
		IsExported: sym.IsExported,
	}

	if includes["signature"] {
		r.Signature = sym.Signature
	}

	if includes["doc"] {
		r.DocComment = sym.DocComment
	}

	r.Complexity = sym.Complexity
	r.ParameterCount = int(sym.ParameterCount)
	r.ReceiverType = sym.ReceiverType

	if includes["flags"] {
		r.FunctionFlags = decodeFunctionFlags(sym.FunctionFlags)
		r.VariableFlags = decodeVariableFlags(sym.VariableFlags)
	}

	if includes["callers"] && tracker != nil {
		r.Callers = tracker.GetCallerNames(sym.ID)
	}

	if includes["callees"] && tracker != nil {
		r.Callees = tracker.GetCalleeNames(sym.ID)
	}

	if includes["type_hierarchy"] && tracker != nil {
		rels := tracker.GetTypeRelationships(sym.ID)
		if rels != nil && rels.HasTypeRelationships() {
			th := &TypeHierarchyInfo{}
			th.Implements = resolveSymbolNames(rels.Implements, s)
			th.ImplementedBy = resolveSymbolNames(rels.ImplementedBy, s)
			th.Extends = resolveSymbolNames(rels.Extends, s)
			th.ExtendedBy = resolveSymbolNames(rels.ExtendedBy, s)
			r.TypeHierarchy = th
		}
	}

	if includes["scope"] && len(sym.ScopeChain) > 0 {
		chain := make([]string, len(sym.ScopeChain))
		for i, sc := range sym.ScopeChain {
			chain[i] = sc.Name
		}
		r.ScopeChain = chain
	}

	if includes["refs"] {
		r.IncomingRefs = len(sym.IncomingRefs)
		r.OutgoingRefs = len(sym.OutgoingRefs)
	}

	if includes["annotations"] && len(sym.Annotations) > 0 {
		r.Annotations = sym.Annotations
	}

	return r
}

// resolveSymbolNames converts SymbolID slices to names
func resolveSymbolNames(ids []types.SymbolID, s *Server) []string {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		sym := s.GetSymbolByID(id)
		if sym != nil {
			names = append(names, sym.Symbol.Name)
		}
	}
	return names
}

// decodeFunctionFlags returns human-readable flag names from the bitfield
func decodeFunctionFlags(flags uint8) []string {
	if flags == 0 {
		return nil
	}
	var result []string
	if flags&types.FunctionFlagAsync != 0 {
		result = append(result, "async")
	}
	if flags&types.FunctionFlagGenerator != 0 {
		result = append(result, "generator")
	}
	if flags&types.FunctionFlagMethod != 0 {
		result = append(result, "method")
	}
	if flags&types.FunctionFlagVariadic != 0 {
		result = append(result, "variadic")
	}
	return result
}

// decodeVariableFlags returns human-readable flag names from the bitfield
func decodeVariableFlags(flags uint8) []string {
	if flags == 0 {
		return nil
	}
	var result []string
	if flags&types.VariableFlagConst != 0 {
		result = append(result, "const")
	}
	if flags&types.VariableFlagStatic != 0 {
		result = append(result, "static")
	}
	if flags&types.VariableFlagPointer != 0 {
		result = append(result, "pointer")
	}
	if flags&types.VariableFlagArray != 0 {
		result = append(result, "array")
	}
	if flags&types.VariableFlagChannel != 0 {
		result = append(result, "channel")
	}
	if flags&types.VariableFlagInterface != 0 {
		result = append(result, "interface")
	}
	return result
}

// computeFileStats computes aggregate statistics for a set of symbols
func computeFileStats(symbols []*types.EnhancedSymbol) *FileStats {
	stats := &FileStats{
		SymbolCount: len(symbols),
	}
	totalComplexity := 0
	complexityCount := 0
	for _, sym := range symbols {
		if sym.IsExported {
			stats.ExportedCount++
		}
		switch sym.Symbol.Type {
		case types.SymbolTypeFunction, types.SymbolTypeMethod:
			stats.FunctionCount++
			if sym.Complexity > 0 {
				totalComplexity += sym.Complexity
				complexityCount++
				if sym.Complexity > stats.MaxComplexity {
					stats.MaxComplexity = sym.Complexity
				}
			}
		case types.SymbolTypeType, types.SymbolTypeStruct, types.SymbolTypeInterface,
			types.SymbolTypeClass, types.SymbolTypeEnum:
			stats.TypeCount++
		}
	}
	if complexityCount > 0 {
		stats.AvgComplexity = float64(totalComplexity) / float64(complexityCount)
	}
	return stats
}

// languageFromPath guesses a programming language from a file extension
func languageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".jsx":
		return "jsx"
	case ".tsx":
		return "tsx"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".kt":
		return "kotlin"
	case ".rb":
		return "ruby"
	case ".swift":
		return "swift"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".h", ".hpp":
		return "c/cpp"
	default:
		return ""
	}
}
