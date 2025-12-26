package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/lci/internal/types"
)

// SideEffectParams defines parameters for side effect analysis queries
type SideEffectParams struct {
	// Query mode: "symbol", "file", "pure", "impure", "category"
	Mode string `json:"mode,omitempty"`

	// Symbol identification (for mode="symbol")
	SymbolID   string `json:"symbol_id,omitempty"`
	SymbolName string `json:"symbol_name,omitempty"`

	// File filtering (for mode="file")
	FilePath string `json:"file_path,omitempty"`
	FileID   int    `json:"file_id,omitempty"`

	// Category filtering (for mode="category")
	Category string `json:"category,omitempty"` // e.g., "io", "global_write", "param_write"

	// Output options
	IncludeReasons    bool `json:"include_reasons,omitempty"`
	IncludeTransitive bool `json:"include_transitive,omitempty"`
	IncludeConfidence bool `json:"include_confidence,omitempty"`

	// Limits
	MaxResults int `json:"max_results,omitempty"`

	// Captures unknown fields for warnings
	Warnings []UnknownField `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling that accepts unknown fields
func (s *SideEffectParams) UnmarshalJSON(data []byte) error {
	type Alias SideEffectParams

	knownFields := map[string]struct{}{
		"mode": {}, "symbol_id": {}, "symbol_name": {},
		"file_path": {}, "file_id": {}, "category": {},
		"include_reasons": {}, "include_transitive": {}, "include_confidence": {},
		"max_results": {},
	}

	_, warnings, err := collectUnknownFields(data, knownFields, nil)
	if err != nil {
		return err
	}

	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*s = SideEffectParams(alias)
	s.Warnings = warnings
	return nil
}

// SideEffectResult represents side effect analysis for a single function
type SideEffectResult struct {
	SymbolID    string  `json:"symbol_id"`
	SymbolName  string  `json:"symbol_name"`
	FilePath    string  `json:"file_path"`
	Line        int     `json:"line"`
	EndLine     int     `json:"end_line,omitempty"`
	IsPure      bool    `json:"is_pure"`
	PurityScore float64 `json:"purity_score,omitempty"`

	// Categories as human-readable strings
	LocalCategories      []string `json:"local_categories,omitempty"`
	TransitiveCategories []string `json:"transitive_categories,omitempty"`

	// Confidence level
	Confidence string `json:"confidence,omitempty"`

	// Reasons for impurity
	Reasons []string `json:"reasons,omitempty"`

	// Access pattern info
	AccessPattern string   `json:"access_pattern,omitempty"`
	Violations    []string `json:"violations,omitempty"`

	// Error handling info
	CanThrow         bool `json:"can_throw,omitempty"`
	ExceptionNeutral bool `json:"exception_neutral,omitempty"`
	ExceptionSafe    bool `json:"exception_safe,omitempty"`
	DeferCount       int  `json:"defer_count,omitempty"`
}

// SideEffectResponse represents the response for side effect queries
type SideEffectResponse struct {
	Results    []SideEffectResult `json:"results"`
	TotalCount int                `json:"total_count"`
	Mode       string             `json:"mode"`
	Summary    *SideEffectSummary `json:"summary,omitempty"`
}

// SideEffectSummary provides aggregate statistics
type SideEffectSummary struct {
	TotalFunctions  int     `json:"total_functions"`
	PureFunctions   int     `json:"pure_functions"`
	ImpureFunctions int     `json:"impure_functions"`
	PurityRatio     float64 `json:"purity_ratio"`

	// Category breakdown
	WithParamWrites   int `json:"with_param_writes,omitempty"`
	WithGlobalWrites  int `json:"with_global_writes,omitempty"`
	WithIOEffects     int `json:"with_io_effects,omitempty"`
	WithThrows        int `json:"with_throws,omitempty"`
	WithExternalCalls int `json:"with_external_calls,omitempty"`
}

// handleSideEffects handles side effect analysis queries
// @lci:labels[mcp-tool-handler,side-effect-analysis,purity-query]
// @lci:category[mcp-api]
func (s *Server) handleSideEffects(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params SideEffectParams
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return createSmartErrorResponse("side_effects", fmt.Errorf("invalid parameters: %w", err), map[string]interface{}{
			"correct_format": map[string]interface{}{
				"mode":        "pure",
				"max_results": 50,
			},
			"available_modes": []string{"symbol", "file", "pure", "impure", "category", "summary"},
			"info_command":    "Run info tool with {\"tool\": \"side_effects\"} for examples",
		})
	}

	// Check index availability
	if available, err := s.checkIndexAvailability(); err != nil {
		return createSmartErrorResponse("side_effects", err, map[string]interface{}{
			"troubleshooting": []string{
				"Verify you're in a project directory with source code",
				"Check file permissions in project directory",
				"Wait for auto-indexing to complete (check index_stats)",
			},
		})
	} else if !available {
		return createErrorResponse("side_effects", errors.New("side effect analysis cannot proceed: index is not available"))
	}

	// Default parameters
	if params.Mode == "" {
		params.Mode = "summary"
	}
	if params.MaxResults == 0 {
		params.MaxResults = 100
	}

	// Get side effect propagator from index
	propagator := s.goroutineIndex.GetSideEffectPropagator()
	if propagator == nil {
		// Side effect tracking may not be enabled
		return createSmartErrorResponse("side_effects", errors.New("side effect analysis not available"), map[string]interface{}{
			"note": "Side effect tracking may not be enabled for this index",
			"hint": "Re-index with side effect tracking enabled",
		})
	}

	// Handle different query modes
	switch params.Mode {
	case "symbol":
		return s.handleSideEffectSymbolQuery(params, propagator)
	case "file":
		return s.handleSideEffectFileQuery(params, propagator)
	case "pure":
		return s.handleSideEffectPureQuery(params, propagator)
	case "impure":
		return s.handleSideEffectImpureQuery(params, propagator)
	case "category":
		return s.handleSideEffectCategoryQuery(params, propagator)
	case "summary":
		return s.handleSideEffectSummary(params, propagator)
	default:
		return createErrorResponse("side_effects", fmt.Errorf("unknown mode: %s (valid: symbol, file, pure, impure, category, summary)", params.Mode))
	}
}

// handleSideEffectSymbolQuery queries side effects for a specific symbol
func (s *Server) handleSideEffectSymbolQuery(params SideEffectParams, propagator SideEffectPropagatorInterface) (*mcp.CallToolResult, error) {
	if params.SymbolID == "" && params.SymbolName == "" {
		return createErrorResponse("side_effects", errors.New("symbol mode requires 'symbol_id' or 'symbol_name'"))
	}

	// Find symbol
	var symbolID types.SymbolID
	if params.SymbolID != "" {
		// Decode symbol ID
		decoded, err := decodeSymbolID(params.SymbolID)
		if err != nil {
			return createErrorResponse("side_effects", fmt.Errorf("invalid symbol_id: %w", err))
		}
		symbolID = decoded
	} else {
		// Look up by name
		symbols := s.goroutineIndex.GetRefTracker().FindSymbolsByName(params.SymbolName)
		if len(symbols) == 0 {
			return createErrorResponse("side_effects", fmt.Errorf("symbol not found: %s", params.SymbolName))
		}
		symbolID = symbols[0].ID
	}

	info := propagator.GetSideEffectInfo(symbolID)
	if info == nil {
		return createJSONResponse(SideEffectResponse{
			Results:    []SideEffectResult{},
			TotalCount: 0,
			Mode:       "symbol",
		})
	}

	result := convertSideEffectInfo(info, params)
	result.SymbolID = params.SymbolID
	if params.SymbolName != "" {
		result.SymbolName = params.SymbolName
	}

	return createJSONResponse(SideEffectResponse{
		Results:    []SideEffectResult{result},
		TotalCount: 1,
		Mode:       "symbol",
	})
}

// handleSideEffectFileQuery queries side effects for all functions in a file
func (s *Server) handleSideEffectFileQuery(params SideEffectParams, propagator SideEffectPropagatorInterface) (*mcp.CallToolResult, error) {
	if params.FilePath == "" && params.FileID == 0 {
		return createErrorResponse("side_effects", errors.New("file mode requires 'file_path' or 'file_id'"))
	}

	// Get all side effects and filter by file
	allEffects := propagator.GetAllSideEffects()
	results := make([]SideEffectResult, 0, params.MaxResults)

	for symbolID, info := range allEffects {
		if info == nil {
			continue
		}

		// Filter by file
		if params.FilePath != "" && info.FilePath != params.FilePath {
			continue
		}
		if params.FileID != 0 {
			fileID := types.FileID(symbolID >> 32)
			if int(fileID) != params.FileID {
				continue
			}
		}

		result := convertSideEffectInfo(info, params)
		result.SymbolID = encodeSymbolID(symbolID)
		results = append(results, result)

		if len(results) >= params.MaxResults {
			break
		}
	}

	return createJSONResponse(SideEffectResponse{
		Results:    results,
		TotalCount: len(results),
		Mode:       "file",
	})
}

// handleSideEffectPureQuery returns all pure functions
func (s *Server) handleSideEffectPureQuery(params SideEffectParams, propagator SideEffectPropagatorInterface) (*mcp.CallToolResult, error) {
	pureSymbols := propagator.GetPureFunctions()
	results := make([]SideEffectResult, 0, minInt(len(pureSymbols), params.MaxResults))

	for i, symbolID := range pureSymbols {
		if i >= params.MaxResults {
			break
		}

		info := propagator.GetSideEffectInfo(symbolID)
		if info == nil {
			continue
		}

		result := convertSideEffectInfo(info, params)
		result.SymbolID = encodeSymbolID(symbolID)
		results = append(results, result)
	}

	return createJSONResponse(SideEffectResponse{
		Results:    results,
		TotalCount: len(pureSymbols),
		Mode:       "pure",
	})
}

// handleSideEffectImpureQuery returns all impure functions
func (s *Server) handleSideEffectImpureQuery(params SideEffectParams, propagator SideEffectPropagatorInterface) (*mcp.CallToolResult, error) {
	impureSymbols := propagator.GetImpureFunctions()
	results := make([]SideEffectResult, 0, minInt(len(impureSymbols), params.MaxResults))

	for i, symbolID := range impureSymbols {
		if i >= params.MaxResults {
			break
		}

		info := propagator.GetSideEffectInfo(symbolID)
		if info == nil {
			continue
		}

		result := convertSideEffectInfo(info, params)
		result.SymbolID = encodeSymbolID(symbolID)
		results = append(results, result)
	}

	return createJSONResponse(SideEffectResponse{
		Results:    results,
		TotalCount: len(impureSymbols),
		Mode:       "impure",
	})
}

// handleSideEffectCategoryQuery filters by side effect category
func (s *Server) handleSideEffectCategoryQuery(params SideEffectParams, propagator SideEffectPropagatorInterface) (*mcp.CallToolResult, error) {
	if params.Category == "" {
		return createErrorResponse("side_effects", errors.New("category mode requires 'category' parameter"))
	}

	// Map category name to SideEffectCategory
	categoryBit := categoryNameToBit(params.Category)
	if categoryBit == types.SideEffectNone {
		return createErrorResponse("side_effects", fmt.Errorf("unknown category: %s (valid: param_write, global_write, io, network, throw, channel, external_call)", params.Category))
	}

	allEffects := propagator.GetAllSideEffects()
	results := make([]SideEffectResult, 0, params.MaxResults)

	for symbolID, info := range allEffects {
		if info == nil {
			continue
		}

		// Check if category matches (local or transitive)
		combined := info.Categories | info.TransitiveCategories
		if combined&categoryBit == 0 {
			continue
		}

		result := convertSideEffectInfo(info, params)
		result.SymbolID = encodeSymbolID(symbolID)
		results = append(results, result)

		if len(results) >= params.MaxResults {
			break
		}
	}

	return createJSONResponse(SideEffectResponse{
		Results:    results,
		TotalCount: len(results),
		Mode:       "category",
	})
}

// handleSideEffectSummary returns aggregate statistics
func (s *Server) handleSideEffectSummary(params SideEffectParams, propagator SideEffectPropagatorInterface) (*mcp.CallToolResult, error) {
	allEffects := propagator.GetAllSideEffects()

	summary := &SideEffectSummary{
		TotalFunctions: len(allEffects),
	}

	for _, info := range allEffects {
		if info == nil {
			continue
		}

		if info.IsPure {
			summary.PureFunctions++
		} else {
			summary.ImpureFunctions++
		}

		combined := info.Categories | info.TransitiveCategories
		if combined&types.SideEffectParamWrite != 0 {
			summary.WithParamWrites++
		}
		if combined&types.SideEffectGlobalWrite != 0 {
			summary.WithGlobalWrites++
		}
		if combined&(types.SideEffectIO|types.SideEffectNetwork|types.SideEffectDatabase) != 0 {
			summary.WithIOEffects++
		}
		if combined&types.SideEffectThrow != 0 {
			summary.WithThrows++
		}
		if combined&types.SideEffectExternalCall != 0 {
			summary.WithExternalCalls++
		}
	}

	if summary.TotalFunctions > 0 {
		summary.PurityRatio = float64(summary.PureFunctions) / float64(summary.TotalFunctions)
	}

	return createJSONResponse(SideEffectResponse{
		Results:    nil,
		TotalCount: summary.TotalFunctions,
		Mode:       "summary",
		Summary:    summary,
	})
}

// Helper functions

func convertSideEffectInfo(info *types.SideEffectInfo, params SideEffectParams) SideEffectResult {
	result := SideEffectResult{
		SymbolName:  info.FunctionName,
		FilePath:    info.FilePath,
		Line:        info.StartLine,
		EndLine:     info.EndLine,
		IsPure:      info.IsPure,
		PurityScore: info.PurityScore,
	}

	// Convert categories to strings
	result.LocalCategories = categoriesToStrings(info.Categories)
	if params.IncludeTransitive {
		result.TransitiveCategories = categoriesToStrings(info.TransitiveCategories)
	}

	if params.IncludeConfidence {
		result.Confidence = confidenceToString(info.Confidence)
	}

	if params.IncludeReasons {
		result.Reasons = info.ImpurityReasons
	}

	// Access pattern
	if info.AccessPattern != nil {
		result.AccessPattern = info.AccessPattern.Pattern.String()
		for _, v := range info.AccessPattern.Violations {
			result.Violations = append(result.Violations, fmt.Sprintf("%s (severity: %.2f)", v.Type.String(), v.Severity))
		}
	}

	// Error handling
	if info.ErrorHandling != nil {
		result.CanThrow = info.ErrorHandling.CanThrow
		result.ExceptionNeutral = info.ErrorHandling.ExceptionNeutral
		result.ExceptionSafe = info.ErrorHandling.ExceptionSafe
		result.DeferCount = info.ErrorHandling.DeferCount
	}

	return result
}

func categoriesToStrings(cat types.SideEffectCategory) []string {
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
	if cat&types.SideEffectUncertain != 0 {
		result = append(result, "uncertain")
	}
	return result
}

func categoryNameToBit(name string) types.SideEffectCategory {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "param_write", "paramwrite", "param-write":
		return types.SideEffectParamWrite
	case "receiver_write", "receiverwrite", "receiver-write":
		return types.SideEffectReceiverWrite
	case "global_write", "globalwrite", "global-write", "global":
		return types.SideEffectGlobalWrite
	case "closure_write", "closurewrite", "closure-write", "closure":
		return types.SideEffectClosureWrite
	case "field_write", "fieldwrite", "field-write":
		return types.SideEffectFieldWrite
	case "io":
		return types.SideEffectIO
	case "database", "db":
		return types.SideEffectDatabase
	case "network", "net":
		return types.SideEffectNetwork
	case "throw", "throws", "panic":
		return types.SideEffectThrow
	case "channel", "chan":
		return types.SideEffectChannel
	case "async":
		return types.SideEffectAsync
	case "external_call", "externalcall", "external-call", "external":
		return types.SideEffectExternalCall
	case "dynamic_call", "dynamiccall", "dynamic-call", "dynamic":
		return types.SideEffectDynamicCall
	case "reflection", "reflect":
		return types.SideEffectReflection
	case "uncertain", "unknown":
		return types.SideEffectUncertain
	default:
		return types.SideEffectNone
	}
}

func confidenceToString(conf types.PurityConfidence) string {
	switch conf {
	case types.ConfidenceProven:
		return "proven"
	case types.ConfidenceHigh:
		return "high"
	case types.ConfidenceMedium:
		return "medium"
	case types.ConfidenceLow:
		return "low"
	default:
		return "none"
	}
}

// SideEffectPropagatorInterface abstracts the propagator for testing
type SideEffectPropagatorInterface interface {
	GetSideEffectInfo(symbolID types.SymbolID) *types.SideEffectInfo
	GetAllSideEffects() map[types.SymbolID]*types.SideEffectInfo
	GetPureFunctions() []types.SymbolID
	GetImpureFunctions() []types.SymbolID
}

// encodeSymbolID converts a SymbolID to a compact string representation
func encodeSymbolID(id types.SymbolID) string {
	return fmt.Sprintf("%d", id)
}

// decodeSymbolID converts a string to a SymbolID
func decodeSymbolID(s string) (types.SymbolID, error) {
	var id uint64
	_, err := fmt.Sscanf(s, "%d", &id)
	if err != nil {
		return 0, err
	}
	return types.SymbolID(id), nil
}

// minInt returns the minimum of two integers (local helper to avoid redeclaration)
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
