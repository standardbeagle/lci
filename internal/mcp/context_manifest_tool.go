package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/standardbeagle/lci/internal/types"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// handleContext handles the context manifest tool - save and load operations
// @lci:labels[mcp-tool-handler,context-manifest,agent-handoff]
// @lci:category[mcp-api]
func (s *Server) handleContext(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse operation parameter
	var params struct {
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return createErrorResponse("context", fmt.Errorf("invalid parameters: %w", err))
	}

	switch params.Operation {
	case "save":
		return s.handleContextSave(ctx, req)
	case "load":
		return s.handleContextLoad(ctx, req)
	default:
		return createErrorResponse("context", fmt.Errorf("invalid operation: %s (must be 'save' or 'load')", params.Operation))
	}
}

// handleContextSave saves a context manifest
func (s *Server) handleContextSave(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params types.SaveParams
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return createErrorResponse("context", fmt.Errorf("invalid save parameters: %w", err))
	}

	// Validate parameters
	if len(params.Refs) == 0 {
		return createErrorResponse("context", errors.New("must provide 'refs' with at least one reference"))
	}

	if params.ToFile == "" && !params.ToString {
		return createErrorResponse("context", errors.New("must provide either 'to_file' or set 'to_string' to true"))
	}

	// Build manifest from explicit refs
	manifest := &types.ContextManifest{
		Task: params.Task,
		Refs: params.Refs,
	}

	// If project root is available, store it
	if s.cfg != nil && s.cfg.Project.Root != "" {
		manifest.ProjectRoot = s.cfg.Project.Root
	}

	// Validate manifest
	if err := manifest.Validate(); err != nil {
		return createErrorResponse("context", fmt.Errorf("invalid manifest: %w", err))
	}

	// Handle append mode
	if params.Append && params.ToFile != "" {
		existingManifest, err := s.loadManifestFromFile(params.ToFile)
		if err != nil {
			// If file doesn't exist, that's fine - just save new manifest
			if !os.IsNotExist(err) {
				return createErrorResponse("context", fmt.Errorf("failed to load existing manifest for append: %w", err))
			}
		} else {
			// Merge refs
			manifest.Refs = append(existingManifest.Refs, manifest.Refs...)
			// Keep existing task if not overridden
			if manifest.Task == "" {
				manifest.Task = existingManifest.Task
			}
		}
	}

	// Compute stats
	stats := manifest.ComputeStats()

	// Save or return as string
	if params.ToFile != "" {
		// Resolve file path relative to project root
		filePath := s.resolveManifestPath(params.ToFile)

		// Save manifest
		if err := s.saveManifestToFile(manifest, filePath); err != nil {
			return createErrorResponse("context", fmt.Errorf("failed to save manifest: %w", err))
		}

		// Return response
		response := types.SaveResponse{
			Saved:     params.ToFile, // Return relative path
			Stats:     stats,
			RefCount:  stats.RefCount,
			FileCount: stats.FileCount,
		}

		return createContextSuccessResponse(response)
	}

	// Return as string
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return createErrorResponse("context", fmt.Errorf("failed to marshal manifest: %w", err))
	}

	response := types.SaveResponse{
		Manifest:  string(manifestJSON),
		Stats:     stats,
		RefCount:  stats.RefCount,
		FileCount: stats.FileCount,
	}

	return createContextSuccessResponse(response)
}

// handleContextLoad loads and hydrates a context manifest
func (s *Server) handleContextLoad(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params types.LoadParams
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return createErrorResponse("context", fmt.Errorf("invalid load parameters: %w", err))
	}

	// Validate parameters
	if params.FromFile == "" && params.FromString == "" {
		return createErrorResponse("context", errors.New("must provide either 'from_file' or 'from_string'"))
	}

	// Default format
	if params.Format == "" {
		params.Format = string(types.FormatFull)
	}

	// Validate format
	format := types.FormatType(params.Format)
	if !format.IsValid() {
		return createErrorResponse("context", fmt.Errorf("invalid format: %s (must be 'full', 'signatures', or 'outline')", params.Format))
	}

	// Load manifest
	var manifest *types.ContextManifest
	var err error

	if params.FromFile != "" {
		manifest, err = s.loadManifestFromFile(params.FromFile)
		if err != nil {
			return createErrorResponse("context", fmt.Errorf("failed to load manifest from file: %w", err))
		}
	} else {
		manifest, err = s.loadManifestFromString(params.FromString)
		if err != nil {
			return createErrorResponse("context", fmt.Errorf("failed to load manifest from string: %w", err))
		}
	}

	// Check if index is available
	if available, err := s.checkIndexAvailability(); err != nil || !available {
		return createErrorResponse("context", fmt.Errorf("index not available for hydration: %w", err))
	}

	// Hydrate manifest
	hydratedContext, err := s.hydrateManifest(ctx, manifest, params)
	if err != nil {
		return createErrorResponse("context", fmt.Errorf("failed to hydrate manifest: %w", err))
	}

	return createContextSuccessResponse(hydratedContext)
}

// saveManifestToFile saves a manifest to a file
func (s *Server) saveManifestToFile(manifest *types.ContextManifest, filePath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal manifest with indentation for readability
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write to file atomically (write to temp file, then rename)
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// loadManifestFromFile loads a manifest from a file
func (s *Server) loadManifestFromFile(relativePath string) (*types.ContextManifest, error) {
	// Resolve file path
	filePath := s.resolveManifestPath(relativePath)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Unmarshal
	var manifest types.ContextManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest JSON: %w", err)
	}

	// Validate
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &manifest, nil
}

// loadManifestFromString loads a manifest from a JSON string
func (s *Server) loadManifestFromString(jsonStr string) (*types.ContextManifest, error) {
	var manifest types.ContextManifest
	if err := json.Unmarshal([]byte(jsonStr), &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest JSON: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &manifest, nil
}

// resolveManifestPath resolves a manifest path relative to project root
func (s *Server) resolveManifestPath(relativePath string) string {
	// If already absolute, return as-is
	if filepath.IsAbs(relativePath) {
		return relativePath
	}

	// Get project root
	projectRoot := s.determineProjectRoot(s.cfg)
	if projectRoot == "" {
		projectRoot = "."
	}

	// Resolve relative to project root
	return filepath.Join(projectRoot, relativePath)
}

// hydrateManifest hydrates a manifest into full context with source code
func (s *Server) hydrateManifest(ctx context.Context, manifest *types.ContextManifest, params types.LoadParams) (*types.HydratedContext, error) {
	hydratedContext := &types.HydratedContext{
		Task:     manifest.Task,
		Refs:     make([]types.HydratedRef, 0, len(manifest.Refs)),
		Warnings: []string{},
	}

	// Apply role filtering
	filteredRefs := s.filterRefsByRole(manifest.Refs, params.Filter, params.Exclude)

	// Track token usage
	totalTokens := 0
	maxTokens := params.MaxTokens
	if maxTokens == 0 {
		maxTokens = int(^uint(0) >> 1) // Max int - no limit
	}

	// Hydrate each ref
	for _, ref := range filteredRefs {
		// Check token budget
		if totalTokens >= maxTokens {
			hydratedContext.Warnings = append(hydratedContext.Warnings,
				fmt.Sprintf("Truncated: reached token limit of %d", maxTokens))
			hydratedContext.Stats.Truncated = true
			break
		}

		// Hydrate the reference
		hydratedRef, tokens, err := s.hydrateReference(ctx, ref, types.FormatType(params.Format))
		if err != nil {
			// Warn but continue
			hydratedContext.Warnings = append(hydratedContext.Warnings,
				fmt.Sprintf("Failed to hydrate %s:%s: %v", ref.F, ref.S, err))
			continue
		}

		totalTokens += tokens
		hydratedContext.Refs = append(hydratedContext.Refs, *hydratedRef)
		hydratedContext.Stats.RefsLoaded++
		hydratedContext.Stats.SymbolsHydrated++

		// Apply expansions
		if len(ref.X) > 0 {
			expandedTokens, err := s.applyExpansions(ctx, ref, hydratedRef, types.FormatType(params.Format), maxTokens-totalTokens)
			if err != nil {
				hydratedContext.Warnings = append(hydratedContext.Warnings,
					fmt.Sprintf("Failed to expand %s:%s: %v", ref.F, ref.S, err))
			} else {
				totalTokens += expandedTokens
				hydratedContext.Stats.ExpansionsApplied += len(ref.X)
			}
		}
	}

	hydratedContext.Stats.TokensApprox = totalTokens

	return hydratedContext, nil
}

// filterRefsByRole filters refs by role
func (s *Server) filterRefsByRole(refs []types.ContextRef, include, exclude []string) []types.ContextRef {
	// If no filtering, return all
	if len(include) == 0 && len(exclude) == 0 {
		return refs
	}

	// Build sets for efficient lookup
	includeSet := make(map[string]struct{})
	excludeSet := make(map[string]struct{})

	for _, role := range include {
		includeSet[role] = struct{}{}
	}
	for _, role := range exclude {
		excludeSet[role] = struct{}{}
	}

	// Filter
	filtered := make([]types.ContextRef, 0, len(refs))
	for _, ref := range refs {
		// If exclude list specified and role is in it, skip
		if len(exclude) > 0 {
			if _, excluded := excludeSet[ref.Role]; excluded {
				continue
			}
		}

		// If include list specified, only include those roles
		if len(include) > 0 {
			if _, included := includeSet[ref.Role]; !included {
				continue
			}
		}

		filtered = append(filtered, ref)
	}

	return filtered
}

// hydrateReference hydrates a single reference into source code
func (s *Server) hydrateReference(ctx context.Context, ref types.ContextRef, format types.FormatType) (*types.HydratedRef, int, error) {
	// Get project root for file resolution
	projectRoot := s.determineProjectRoot(s.cfg)
	if projectRoot == "" {
		projectRoot = "."
	}

	// Create expansion engine with access to index components
	engine := NewExpansionEngine(
		s.goroutineIndex.GetRefTracker(),
		s.goroutineIndex,
	)

	// Use expansion engine to hydrate the reference
	return engine.HydrateReference(ctx, ref, format, projectRoot)
}

// applyExpansions applies expansion directives to a hydrated reference
func (s *Server) applyExpansions(ctx context.Context, ref types.ContextRef, hydratedRef *types.HydratedRef, format types.FormatType, remainingTokens int) (int, error) {
	// Get project root for file resolution
	projectRoot := s.determineProjectRoot(s.cfg)
	if projectRoot == "" {
		projectRoot = "."
	}

	// Create expansion engine with access to index components
	engine := NewExpansionEngine(
		s.goroutineIndex.GetRefTracker(),
		s.goroutineIndex,
	)

	// Use expansion engine to apply expansions
	return engine.ApplyExpansions(ctx, ref, hydratedRef, format, remainingTokens, projectRoot)
}

// createContextSuccessResponse creates a successful tool response for context operations
func createContextSuccessResponse(result interface{}) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(data)},
		},
	}, nil
}
