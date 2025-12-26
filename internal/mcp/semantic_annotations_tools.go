package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SemanticAnnotationsParams defines parameters for semantic annotations queries
type SemanticAnnotationsParams struct {
	Label             string         `json:"label,omitempty"`              // Query by label
	Category          string         `json:"category,omitempty"`           // Query by category
	MinStrength       float64        `json:"min_strength,omitempty"`       // Minimum propagation strength
	IncludeDirect     bool           `json:"include_direct,omitempty"`     // Include direct annotations
	IncludePropagated bool           `json:"include_propagated,omitempty"` // Include propagated labels
	Max               int            `json:"max_results,omitempty"`        // Limit results
	Warnings          []UnknownField `json:"-"`                            // Captures unknown fields
}

// UnmarshalJSON implements custom unmarshaling that accepts unknown fields
func (s *SemanticAnnotationsParams) UnmarshalJSON(data []byte) error {
	type Alias SemanticAnnotationsParams // Type alias to avoid recursion

	// Define known fields
	knownFields := map[string]struct{}{
		"label": {}, "category": {}, "min_strength": {},
		"include_direct": {}, "include_propagated": {}, "max_results": {},
	}

	_, warnings, err := collectUnknownFields(data, knownFields, nil)
	if err != nil {
		return err
	}

	// Now unmarshal into the actual struct
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*s = SemanticAnnotationsParams(alias)
	s.Warnings = warnings
	return nil
}

// SemanticAnnotationResult represents a symbol with semantic annotations
type SemanticAnnotationResult struct {
	SymbolName string `json:"symbol_name"`
	FileID     int    `json:"file_id"`
	SymbolID   string `json:"symbol_id"`
	FilePath   string `json:"file_path,omitempty"`
	Line       int    `json:"line"`

	// Direct annotations
	DirectLabels []string          `json:"direct_labels,omitempty"`
	Category     string            `json:"category,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`

	// Propagated labels
	PropagatedLabels []PropagatedLabelInfo `json:"propagated_labels,omitempty"`
}

// PropagatedLabelInfo contains details about a propagated label
type PropagatedLabelInfo struct {
	Label      string  `json:"label"`
	Strength   float64 `json:"strength"`
	Hops       int     `json:"hops"`
	SourceName string  `json:"source_name,omitempty"`
	SourceFile string  `json:"source_file,omitempty"`
}

// SemanticAnnotationsResponse represents a response containing semantic annotations
type SemanticAnnotationsResponse struct {
	Annotations []SemanticAnnotationResult `json:"annotations"`
	TotalCount  int                        `json:"total_count"`
}

// handleSemanticAnnotations queries symbols by semantic labels and categories
// @lci:labels[mcp-tool-handler,semantic-annotations,label-query,graph-propagation]
// @lci:category[mcp-api]
func (s *Server) handleSemanticAnnotations(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Manual deserialization to avoid "unknown field" errors
	var saParams SemanticAnnotationsParams
	if err := json.Unmarshal(req.Params.Arguments, &saParams); err != nil {
		return createSmartErrorResponse("semantic_annotations", fmt.Errorf("invalid parameters: %w", err), map[string]interface{}{
			"correct_format": map[string]interface{}{
				"label":    "critical-bug",
				"category": "security",
			},
			"info_command": "Run info tool with {\"tool\": \"semantic_annotations\"} for examples",
		})
	}

	args := saParams

	// Validate input
	if args.Label == "" && args.Category == "" {
		return createErrorResponse("semantic_annotations", errors.New("must specify either 'label' or 'category'"))
	}

	// FIX: Check if index is available BEFORE accessing it
	// This prevents returning empty results when index unavailable
	if available, err := s.checkIndexAvailability(); err != nil {
		return createSmartErrorResponse("semantic_annotations", err, map[string]interface{}{
			"troubleshooting": []string{
				"Verify you're in a project directory with source code",
				"Check file permissions in project directory",
				"Review .lci.kdl configuration for errors",
				"Wait for auto-indexing to complete (check index_stats)",
			},
		})
	} else if !available {
		return createErrorResponse("semantic_annotations", errors.New("semantic annotations cannot proceed: index is not available"))
	}

	// Default parameters
	if args.Max == 0 {
		args.Max = 100
	}
	if !args.IncludeDirect && !args.IncludePropagated {
		// Default: include both
		args.IncludeDirect = true
		args.IncludePropagated = true
	}

	// Get semantic annotator
	semanticAnnotator := s.goroutineIndex.GetSemanticAnnotator()
	if semanticAnnotator == nil {
		return createErrorResponse("semantic_annotations", errors.New("semantic annotator not available - index may not be built"))
	}

	// Pre-allocate results slice to avoid reallocations during append
	// Use args.Max as the capacity, with a minimum of 1 and a reasonable maximum
	capacity := args.Max
	if capacity <= 0 {
		capacity = 100 // Default capacity
	} else if capacity > 10000 {
		capacity = 10000 // Cap to prevent excessive allocation
	}
	results := make([]SemanticAnnotationResult, 0, capacity)

	// Query by label
	if args.Label != "" {
		// Get direct annotations
		if args.IncludeDirect {
			annotatedSymbols := semanticAnnotator.GetSymbolsByLabel(args.Label)
			for _, annotated := range annotatedSymbols {
				if len(results) >= args.Max {
					break
				}

				result := SemanticAnnotationResult{
					SymbolName:   annotated.Symbol.Name,
					FileID:       int(annotated.FileID),
					SymbolID:     fmt.Sprintf("%d", annotated.SymbolID),
					FilePath:     annotated.FilePath,
					Line:         annotated.Symbol.Line,
					DirectLabels: annotated.Annotation.Labels,
					Category:     annotated.Annotation.Category,
					Tags:         annotated.Annotation.Tags,
				}
				results = append(results, result)
			}
		}

		// Get propagated labels if requested
		if args.IncludePropagated {
			graphPropagator := s.goroutineIndex.GetGraphPropagator()
			if graphPropagator != nil {
				// Use GetSymbolsWithLabel for efficient propagated label lookup
				propagatedSymbols := graphPropagator.GetSymbolsWithLabel(args.Label, args.MinStrength)

				for _, annotated := range propagatedSymbols {
					if len(results) >= args.Max {
						break
					}

					// Check if already in results (might be in direct annotations)
					symbolIDStr := fmt.Sprintf("%d", annotated.SymbolID)
					found := false
					for i := range results {
						if results[i].SymbolID == symbolIDStr {
							// Already have this symbol, just add propagated label info
							propagatedLabels := graphPropagator.GetPropagatedLabels(annotated.SymbolID)
							for _, pLabel := range propagatedLabels {
								if pLabel.Label == args.Label && pLabel.Strength >= args.MinStrength {
									results[i].PropagatedLabels = append(results[i].PropagatedLabels, PropagatedLabelInfo{
										Label:      pLabel.Label,
										Strength:   pLabel.Strength,
										Hops:       pLabel.Hops,
										SourceName: "", // Would need symbol lookup to get source name
									})
								}
							}
							found = true
							break
						}
					}

					if !found {
						// New symbol with propagated label
						result := SemanticAnnotationResult{
							SymbolName: annotated.Symbol.Name,
							FileID:     int(annotated.FileID),
							SymbolID:   symbolIDStr,
							FilePath:   annotated.FilePath,
							Line:       annotated.Symbol.Line,
						}

						// Add propagated label information
						propagatedLabels := graphPropagator.GetPropagatedLabels(annotated.SymbolID)
						for _, pLabel := range propagatedLabels {
							if pLabel.Label == args.Label && pLabel.Strength >= args.MinStrength {
								result.PropagatedLabels = append(result.PropagatedLabels, PropagatedLabelInfo{
									Label:      pLabel.Label,
									Strength:   pLabel.Strength,
									Hops:       pLabel.Hops,
									SourceName: "", // Would need symbol lookup to get source name
								})
							}
						}

						results = append(results, result)
					}
				}
			}
		}
	}

	// Query by category
	if args.Category != "" && args.IncludeDirect {
		annotatedSymbols := semanticAnnotator.GetSymbolsByCategory(args.Category)
		for _, annotated := range annotatedSymbols {
			if len(results) >= args.Max {
				break
			}

			// Check if already in results
			found := false
			for i := range results {
				if results[i].SymbolID == fmt.Sprintf("%d", annotated.SymbolID) {
					found = true
					break
				}
			}

			if !found {
				result := SemanticAnnotationResult{
					SymbolName:   annotated.Symbol.Name,
					FileID:       int(annotated.FileID),
					SymbolID:     fmt.Sprintf("%d", annotated.SymbolID),
					FilePath:     annotated.FilePath,
					Line:         annotated.Symbol.Line,
					DirectLabels: annotated.Annotation.Labels,
					Category:     annotated.Annotation.Category,
					Tags:         annotated.Annotation.Tags,
				}
				results = append(results, result)
			}
		}
	}

	// Create response with compact format
	response := SemanticAnnotationsResponse{
		Annotations: results,
		TotalCount:  len(results),
	}

	// Use compact format by default (no backward compatibility)
	includeContext := false                   // No context needed for annotations
	includeMetadata := args.IncludePropagated // Include metadata if propagated labels requested
	return createCompactResponse(response, includeContext, includeMetadata)
}
