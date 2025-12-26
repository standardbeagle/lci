package tools

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// EntityIDGenerator creates stable, reproducible entity IDs for all code entities
// Format: {entity_type}:{identifier}:{location_info}
type EntityIDGenerator struct {
	rootPath string
}

// NewEntityIDGenerator creates a new ID generator with the repository root path
func NewEntityIDGenerator(rootPath string) *EntityIDGenerator {
	// Normalize root path - ensure no trailing slash
	rootPath = strings.TrimSuffix(rootPath, "/")
	rootPath = strings.TrimSuffix(rootPath, "\\")

	return &EntityIDGenerator{
		rootPath: rootPath,
	}
}

// GetModuleID creates a module ID: module:<name>:<relative_path>
func (g *EntityIDGenerator) GetModuleID(name, absPath string) string {
	relPath := g.makeRelativePath(absPath)
	// Sanitize module name for ID
	safeName := sanitizeForID(name)
	return fmt.Sprintf("module:%s:%s", safeName, relPath)
}

// GetFileID creates a file ID: file:<filename>:<relative_path>
func (g *EntityIDGenerator) GetFileID(absPath string) string {
	filename := filepath.Base(absPath)
	relPath := g.makeRelativePath(absPath)
	return fmt.Sprintf("file:%s:%s", filename, relPath)
}

// GetSymbolID creates a symbol ID based on symbol type: symbol:<type>_<name>:<file>:<line>:<column>
func (g *EntityIDGenerator) GetSymbolID(symbolType, name, file string, line, column int) string {
	filename := filepath.Base(file)
	safeName := sanitizeForID(name)
	return fmt.Sprintf("symbol:%s_%s:%s:%d:%d", symbolType, safeName, filename, line, column)
}

// NormalizeSymbolType converts a symbol type string to entity ID format.
// This provides a single source of truth for symbol type normalization.
// Returns empty string for unknown/invalid types.
//
// This function ensures consistency between the types package EntityID methods
// and the tools package EntityIDGenerator.
func NormalizeSymbolType(symbolType string) string {
	switch symbolType {
	case "function":
		return "func"
	case "class":
		return "class"
	case "method":
		return "method"
	case "struct":
		return "struct"
	case "interface":
		return "interface"
	case "variable":
		return "var"
	case "constant":
		return "const"
	case "enum":
		return "enum"
	case "type":
		return "type"
	case "property":
		return "property"
	case "field":
		return "field"
	case "module":
		return "module"
	case "namespace":
		return "namespace"
	case "operator":
		return "operator"
	default:
		return ""
	}
}

// GetReferenceID creates a reference ID: reference:<type>_<symbol_id>:<file>:<line>:<column>
func (g *EntityIDGenerator) GetReferenceID(refType, symbolID, file string, line, column int) string {
	filename := filepath.Base(file)
	return fmt.Sprintf("reference:%s_%s:%s:%d:%d", refType, symbolID, filename, line, column)
}

// GetCallsiteID creates a callsite ID: reference:call_<symbol_name>:<file>:<line>:<column>
func (g *EntityIDGenerator) GetCallsiteID(symbolName, file string, line, column int) string {
	return g.GetReferenceID("call", symbolName, file, line, column)
}

// GetUsageID creates a usage ID: reference:use_<symbol_name>:<file>:<line>:<column>
func (g *EntityIDGenerator) GetUsageID(symbolName, file string, line, column int) string {
	return g.GetReferenceID("use", symbolName, file, line, column)
}

// makeRelativePath creates a relative path from the repository root
func (g *EntityIDGenerator) makeRelativePath(absPath string) string {
	relPath := strings.TrimPrefix(absPath, g.rootPath)
	relPath = strings.TrimPrefix(relPath, "/")
	relPath = strings.TrimPrefix(relPath, "\\")
	return relPath
}

// sanitizeForID converts names to safe ID components
func sanitizeForID(name string) string {
	// Replace spaces and special characters with underscores
	var result strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			result.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '.' {
			result.WriteRune('_')
		}
		// Skip other special characters
	}

	safe := result.String()

	// Ensure doesn't start with digit
	if len(safe) > 0 && unicode.IsDigit(rune(safe[0])) {
		safe = "_" + safe
	}

	// Handle empty result
	if safe == "" {
		safe = "unnamed"
	}

	return safe
}

// ParseEntityID extracts components from an entity ID
func ParseEntityID(entityID string) (entityType, identifier, location string, err error) {
	parts := strings.SplitN(entityID, ":", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid entity ID format: %s", entityID)
	}

	return parts[0], parts[1], parts[2], nil
}

// ExtractSymbolInfo extracts symbol information from a symbol ID
func ExtractSymbolInfo(symbolID string) (symbolType, name, file string, line, column int, err error) {
	// Expected format: symbol:<type>_<name>:<file>:<line>:<column>
	parts := strings.Split(symbolID, ":")
	if len(parts) != 5 || parts[0] != "symbol" {
		return "", "", "", 0, 0, fmt.Errorf("invalid symbol ID format: %s", symbolID)
	}

	// Extract type and name from identifier part
	identifier := parts[1]
	typeNameParts := strings.SplitN(identifier, "_", 2)
	if len(typeNameParts) != 2 {
		return "", "", "", 0, 0, fmt.Errorf("invalid symbol identifier: %s", identifier)
	}

	symbolType = typeNameParts[0]
	name = typeNameParts[1]
	file = parts[2]

	// Parse line and column
	_, err = fmt.Sscanf(parts[3]+":"+parts[4], "%d:%d", &line, &column)
	if err != nil {
		return "", "", "", 0, 0, fmt.Errorf("invalid location in symbol ID: %s:%s", parts[3], parts[4])
	}

	return symbolType, name, file, line, column, nil
}

// EntityType constants for consistent usage
const (
	EntityTypeModule    = "module"
	EntityTypeFile      = "file"
	EntityTypeSymbol    = "symbol"
	EntityTypeReference = "reference"

	// Symbol subtypes
	SymbolTypeFunction  = "func"
	SymbolTypeMethod    = "method"
	SymbolTypeStruct    = "struct"
	SymbolTypeInterface = "interface"
	SymbolTypeVariable  = "var"
	SymbolTypeConstant  = "const"
	SymbolTypeEnum      = "enum"
	SymbolTypeType      = "type"

	// Reference subtypes
	ReferenceTypeCall     = "call"
	ReferenceTypeUse      = "use"
	ReferenceTypeImpl     = "impl"
	ReferenceTypeOverride = "override"
)

// IsValidEntityID validates an entity ID format
func IsValidEntityID(entityID string) bool {
	if entityID == "" {
		return false
	}

	parts := strings.Split(entityID, ":")
	// Basic entity IDs have exactly 3 parts (type:identifier:location)
	// Symbol and reference IDs have 5 parts (type:identifier:file:line:column)
	if len(parts) != 3 && len(parts) != 5 {
		return false
	}

	entityType := parts[0]
	switch entityType {
	case EntityTypeModule, EntityTypeFile, EntityTypeSymbol, EntityTypeReference:
		return true
	default:
		return false
	}
}

// GetEntityType extracts the entity type from an ID
func GetEntityType(entityID string) string {
	parts := strings.Split(entityID, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
