package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/standardbeagle/lci/internal/types"
)

// Global cached patterns - compiled once and shared across all ComponentDetector instances
var (
	globalPatterns     *cachedComponentPatterns
	globalPatternsOnce sync.Once
)

// cachedComponentPatterns holds pre-compiled regex patterns
type cachedComponentPatterns struct {
	namingPatterns map[types.ComponentType][]*regexp.Regexp
	pathPatterns   map[types.ComponentType][]*regexp.Regexp
	symbolPatterns map[types.ComponentType][]SymbolRule
	languageRules  map[string]LanguageRules
}

// ComponentDetector identifies semantic components in codebases using AST analysis and naming patterns
type ComponentDetector struct {
	// Pattern-based rules for component classification (shared globally)
	namingPatterns map[types.ComponentType][]*regexp.Regexp
	pathPatterns   map[types.ComponentType][]*regexp.Regexp

	// Symbol-based classification rules (shared globally)
	symbolPatterns map[types.ComponentType][]SymbolRule

	// Language-specific detection rules (shared globally)
	languageRules map[string]LanguageRules
}

// SymbolRule defines rules for classifying components based on symbols
type SymbolRule struct {
	SymbolKind      types.SymbolKind
	NamePattern     *regexp.Regexp
	TypePattern     *regexp.Regexp // For type annotations, return types, etc.
	RequiredImports []string       // Required import patterns
	Confidence      float64        // Rule confidence multiplier
}

// LanguageRules contains language-specific detection rules
type LanguageRules struct {
	Language          string
	MainFunctionNames []string // e.g., ["main", "Main", "_main"]
	HandlerSuffixes   []string // e.g., ["Handler", "Controller", "Endpoint"]
	ViewSuffixes      []string // e.g., ["Component", "View", "Template"]
	ModelSuffixes     []string // e.g., ["Model", "Entity", "Schema"]
	TestPatterns      []string // e.g., ["test", "spec", "_test"]
	ConfigNames       []string // e.g., ["config", "settings", "env"]
}

// NewComponentDetector creates a new component detector with shared cached rules
func NewComponentDetector() *ComponentDetector {
	// Initialize patterns once globally
	globalPatternsOnce.Do(func() {
		globalPatterns = &cachedComponentPatterns{
			namingPatterns: make(map[types.ComponentType][]*regexp.Regexp),
			pathPatterns:   make(map[types.ComponentType][]*regexp.Regexp),
			symbolPatterns: make(map[types.ComponentType][]SymbolRule),
			languageRules:  make(map[string]LanguageRules),
		}
		initializeGlobalPatterns()
	})

	// Return detector that shares the global patterns
	return &ComponentDetector{
		namingPatterns: globalPatterns.namingPatterns,
		pathPatterns:   globalPatterns.pathPatterns,
		symbolPatterns: globalPatterns.symbolPatterns,
		languageRules:  globalPatterns.languageRules,
	}
}

// initializeGlobalPatterns sets up the global component detection rules (called once)
func initializeGlobalPatterns() {
	// Initialize language-specific rules
	globalPatterns.languageRules["go"] = LanguageRules{
		Language:          "go",
		MainFunctionNames: []string{"main", "init"},
		HandlerSuffixes:   []string{"Handler", "Controller", "Endpoint", "API"},
		ViewSuffixes:      []string{"View", "Template", "Renderer", "UI"},
		ModelSuffixes:     []string{"Model", "Entity", "Struct", "Type"},
		TestPatterns:      []string{"test", "_test"},
		ConfigNames:       []string{"config", "Config", "settings", "Settings", "env", "Env"},
	}

	globalPatterns.languageRules["javascript"] = LanguageRules{
		Language:          "javascript",
		MainFunctionNames: []string{"main", "start", "run"},
		HandlerSuffixes:   []string{"Handler", "Controller", "Route", "API", "Endpoint"},
		ViewSuffixes:      []string{"Component", "View", "Template", "Widget"},
		ModelSuffixes:     []string{"Model", "Schema", "Entity", "Type"},
		TestPatterns:      []string{"test", "spec"},
		ConfigNames:       []string{"config", "settings", "env"},
	}

	globalPatterns.languageRules["typescript"] = globalPatterns.languageRules["javascript"]

	// Initialize naming patterns
	initializeNamingPatterns()
	initializePathPatterns()
	initializeSymbolPatterns()
}

// initializeNamingPatterns sets up naming pattern rules (called once globally)
func initializeNamingPatterns() {
	// Entry points
	globalPatterns.namingPatterns[types.ComponentTypeEntryPoint] = []*regexp.Regexp{
		regexp.MustCompile(`^main$`),
		regexp.MustCompile(`^Main$`),
		regexp.MustCompile(`^init$`),
		regexp.MustCompile(`^_main$`),
		regexp.MustCompile(`^bootstrap$`),
		regexp.MustCompile(`^start$`),
		regexp.MustCompile(`^run$`),
	}

	// API Handlers
	globalPatterns.namingPatterns[types.ComponentTypeAPIHandler] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*handler$`),
		regexp.MustCompile(`(?i).*controller$`),
		regexp.MustCompile(`(?i).*endpoint$`),
		regexp.MustCompile(`(?i).*route$`),
		regexp.MustCompile(`(?i).*api$`),
		regexp.MustCompile(`(?i).*rest$`),
		regexp.MustCompile(`(?i).*graphql$`),
		regexp.MustCompile(`(?i)handle.*`),
		regexp.MustCompile(`(?i)serve.*`),
	}

	// View Components
	globalPatterns.namingPatterns[types.ComponentTypeViewController] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*component$`),
		regexp.MustCompile(`(?i).*view$`),
		regexp.MustCompile(`(?i).*template$`),
		regexp.MustCompile(`(?i).*renderer$`),
		regexp.MustCompile(`(?i).*ui$`),
		regexp.MustCompile(`(?i).*widget$`),
		regexp.MustCompile(`(?i)render.*`),
		regexp.MustCompile(`(?i)display.*`),
	}

	// Controllers
	globalPatterns.namingPatterns[types.ComponentTypeController] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*controller$`),
		regexp.MustCompile(`(?i).*manager$`),
		regexp.MustCompile(`(?i).*coordinator$`),
		regexp.MustCompile(`(?i).*orchestrator$`),
		regexp.MustCompile(`(?i).*facade$`),
	}

	// Data Models
	globalPatterns.namingPatterns[types.ComponentTypeDataModel] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*model$`),
		regexp.MustCompile(`(?i).*entity$`),
		regexp.MustCompile(`(?i).*schema$`),
		regexp.MustCompile(`(?i).*type$`),
		regexp.MustCompile(`(?i).*data$`),
		regexp.MustCompile(`(?i).*dto$`),
		regexp.MustCompile(`(?i).*struct$`),
	}

	// Configuration
	globalPatterns.namingPatterns[types.ComponentTypeConfiguration] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*config$`),
		regexp.MustCompile(`(?i).*settings$`),
		regexp.MustCompile(`(?i).*env$`),
		regexp.MustCompile(`(?i).*options$`),
		regexp.MustCompile(`(?i).*params$`),
	}

	// Tests
	globalPatterns.namingPatterns[types.ComponentTypeTest] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*test$`),
		regexp.MustCompile(`(?i).*spec$`),
		regexp.MustCompile(`(?i)test.*`),
		regexp.MustCompile(`(?i)mock.*`),
		regexp.MustCompile(`(?i).*mock$`),
	}

	// Services
	globalPatterns.namingPatterns[types.ComponentTypeService] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*service$`),
		regexp.MustCompile(`(?i).*provider$`),
		regexp.MustCompile(`(?i).*client$`),
		regexp.MustCompile(`(?i).*gateway$`),
	}

	// Repositories
	globalPatterns.namingPatterns[types.ComponentTypeRepository] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*repository$`),
		regexp.MustCompile(`(?i).*repo$`),
		regexp.MustCompile(`(?i).*dao$`),
		regexp.MustCompile(`(?i).*store$`),
		regexp.MustCompile(`(?i).*storage$`),
	}

	// Middleware
	globalPatterns.namingPatterns[types.ComponentTypeMiddleware] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*middleware$`),
		regexp.MustCompile(`(?i).*filter$`),
		regexp.MustCompile(`(?i).*interceptor$`),
		regexp.MustCompile(`(?i).*guard$`),
	}

	// Utilities
	globalPatterns.namingPatterns[types.ComponentTypeUtility] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*util$`),
		regexp.MustCompile(`(?i).*utils$`),
		regexp.MustCompile(`(?i).*helper$`),
		regexp.MustCompile(`(?i).*helpers$`),
		regexp.MustCompile(`(?i).*common$`),
		regexp.MustCompile(`(?i).*shared$`),
	}
}

// initializePathPatterns sets up file path pattern rules
func initializePathPatterns() {
	// Entry points - usually in root or cmd directories
	globalPatterns.pathPatterns[types.ComponentTypeEntryPoint] = []*regexp.Regexp{
		regexp.MustCompile(`main\.go$`),
		regexp.MustCompile(`cmd/.*\.go$`),
		regexp.MustCompile(`bin/.*\.go$`),
		regexp.MustCompile(`index\.(js|ts)$`),
		regexp.MustCompile(`app\.(js|ts)$`),
		regexp.MustCompile(`server\.(js|ts|go)$`),
	}

	// API Handlers
	globalPatterns.pathPatterns[types.ComponentTypeAPIHandler] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*/handlers?/.*`),
		regexp.MustCompile(`(?i).*/controllers?/.*`),
		regexp.MustCompile(`(?i).*/api/.*`),
		regexp.MustCompile(`(?i).*/routes?/.*`),
		regexp.MustCompile(`(?i).*/endpoints?/.*`),
	}

	// Views
	globalPatterns.pathPatterns[types.ComponentTypeViewController] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*/views?/.*`),
		regexp.MustCompile(`(?i).*/components?/.*`),
		regexp.MustCompile(`(?i).*/templates?/.*`),
		regexp.MustCompile(`(?i).*/ui/.*`),
		regexp.MustCompile(`(?i).*/tui/.*`), // Terminal UI
		regexp.MustCompile(`(?i).*/widgets?/.*`),
	}

	// Models
	globalPatterns.pathPatterns[types.ComponentTypeDataModel] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*/models?/.*`),
		regexp.MustCompile(`(?i).*/entities/.*`),
		regexp.MustCompile(`(?i).*/schemas?/.*`),
		regexp.MustCompile(`(?i).*/types?/.*`),
		regexp.MustCompile(`(?i).*/data/.*`),
	}

	// Configuration
	globalPatterns.pathPatterns[types.ComponentTypeConfiguration] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*/config/.*`),
		regexp.MustCompile(`(?i).*/configs?/.*`),
		regexp.MustCompile(`(?i).*/settings/.*`),
		regexp.MustCompile(`(?i).*/env/.*`),
	}

	// Tests
	globalPatterns.pathPatterns[types.ComponentTypeTest] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*test.*\.(go|js|ts)$`),
		regexp.MustCompile(`(?i).*spec.*\.(js|ts)$`),
		regexp.MustCompile(`(?i).*/tests?/.*`),
		regexp.MustCompile(`(?i).*_test\.go$`),
		regexp.MustCompile(`(?i).*\.test\.(js|ts)$`),
		regexp.MustCompile(`(?i).*\.spec\.(js|ts)$`),
	}

	// Services
	globalPatterns.pathPatterns[types.ComponentTypeService] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*/services?/.*`),
		regexp.MustCompile(`(?i).*/providers?/.*`),
		regexp.MustCompile(`(?i).*/clients?/.*`),
	}

	// Utilities
	globalPatterns.pathPatterns[types.ComponentTypeUtility] = []*regexp.Regexp{
		regexp.MustCompile(`(?i).*/utils?/.*`),
		regexp.MustCompile(`(?i).*/helpers?/.*`),
		regexp.MustCompile(`(?i).*/common/.*`),
		regexp.MustCompile(`(?i).*/shared/.*`),
		regexp.MustCompile(`(?i).*/lib/.*`),
	}
}

// initializeSymbolPatterns sets up symbol-based classification rules
func initializeSymbolPatterns() {
	// Entry points - look for main functions
	globalPatterns.symbolPatterns[types.ComponentTypeEntryPoint] = []SymbolRule{
		{
			SymbolKind:  types.SymbolKindFunction,
			NamePattern: regexp.MustCompile(`^main$`),
			Confidence:  0.95,
		},
		{
			SymbolKind:  types.SymbolKindFunction,
			NamePattern: regexp.MustCompile(`^init$`),
			Confidence:  0.8,
		},
	}

	// API Handlers - look for HTTP-related functions
	globalPatterns.symbolPatterns[types.ComponentTypeAPIHandler] = []SymbolRule{
		{
			SymbolKind:  types.SymbolKindFunction,
			NamePattern: regexp.MustCompile(`(?i)handle.*`),
			Confidence:  0.8,
		},
		{
			SymbolKind:  types.SymbolKindFunction,
			NamePattern: regexp.MustCompile(`(?i)serve.*`),
			Confidence:  0.8,
		},
		{
			SymbolKind:  types.SymbolKindMethod,
			NamePattern: regexp.MustCompile(`(?i).*(get|post|put|delete|patch).*`),
			Confidence:  0.7,
		},
	}

	// Data Models - look for structs, interfaces, types
	globalPatterns.symbolPatterns[types.ComponentTypeDataModel] = []SymbolRule{
		{
			SymbolKind:  types.SymbolKindStruct,
			NamePattern: regexp.MustCompile(`.*`), // Any struct
			Confidence:  0.6,
		},
		{
			SymbolKind:  types.SymbolKindInterface,
			NamePattern: regexp.MustCompile(`.*`), // Any interface
			Confidence:  0.6,
		},
		{
			SymbolKind:  types.SymbolKindType,
			NamePattern: regexp.MustCompile(`.*`), // Any type definition
			Confidence:  0.5,
		},
	}
}

// DetectComponents analyzes files and symbols to identify semantic components
func (cd *ComponentDetector) DetectComponents(files map[types.FileID]string, symbols map[types.FileID][]types.Symbol, options types.ComponentSearchOptions) ([]types.ComponentInfo, error) {
	// Set defaults
	if options.MinConfidence == 0 {
		options.MinConfidence = 0.5
	}
	if options.MaxResults == 0 {
		options.MaxResults = 100
	}

	var components []types.ComponentInfo

	// Analyze each file
	for fileID, filePath := range files {
		fileSymbols := symbols[fileID]
		language := cd.detectLanguage(filePath)

		// Skip tests unless explicitly requested
		if !options.IncludeTests && cd.isTestFile(filePath, language) {
			continue
		}

		// Detect components in this file
		fileComponents := cd.analyzeFile(fileID, filePath, language, fileSymbols)

		// Filter by requested types and confidence
		for _, component := range fileComponents {
			if component.Confidence >= options.MinConfidence {
				// Check if this component type is requested
				if len(options.Types) == 0 || cd.containsComponentType(options.Types, component.Type) {
					// Check language filter
					if options.Language == "" || component.Language == options.Language {
						components = append(components, component)
					}
				}
			}
		}
	}

	// Sort by confidence (highest first)
	sort.Slice(components, func(i, j int) bool {
		if components[i].Confidence == components[j].Confidence {
			return components[i].Name < components[j].Name // Secondary sort by name
		}
		return components[i].Confidence > components[j].Confidence
	})

	// Limit results
	if len(components) > options.MaxResults {
		components = components[:options.MaxResults]
	}

	return components, nil
}

// analyzeFile analyzes a single file for semantic components
func (cd *ComponentDetector) analyzeFile(fileID types.FileID, filePath, language string, symbols []types.Symbol) []types.ComponentInfo {
	var components []types.ComponentInfo

	// Check for file-level component classification
	fileComponent := cd.classifyByFilePath(fileID, filePath, language, symbols)
	if fileComponent != nil {
		components = append(components, *fileComponent)
	}

	// Analyze individual symbols
	for _, symbol := range symbols {
		symbolComponent := cd.classifyBySymbol(fileID, filePath, language, symbol)
		if symbolComponent != nil {
			components = append(components, *symbolComponent)
		}
	}

	// If no specific components found, create a generic one based on file patterns
	if len(components) == 0 {
		genericComponent := cd.createGenericComponent(fileID, filePath, language, symbols)
		if genericComponent != nil {
			components = append(components, *genericComponent)
		}
	}

	return components
}

// classifyByFilePath classifies a component based on file path patterns
func (cd *ComponentDetector) classifyByFilePath(fileID types.FileID, filePath, language string, symbols []types.Symbol) *types.ComponentInfo {
	baseName := filepath.Base(filePath)

	// Check path patterns for each component type
	for componentType, patterns := range cd.pathPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(filePath) {
				return &types.ComponentInfo{
					Type:        componentType,
					Name:        cd.generateComponentName(baseName, componentType),
					FilePath:    filePath,
					FileID:      fileID,
					Language:    language,
					Description: cd.generateDescription(componentType, baseName),
					Symbols:     symbols,
					Patterns:    []string{pattern.String()},
					Confidence:  cd.calculatePathConfidence(componentType, filePath),
					Evidence:    []string{"File path matches pattern: " + pattern.String()},
					Metadata:    map[string]interface{}{"classification_method": "file_path"},
				}
			}
		}
	}

	// Check naming patterns on the base filename
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	for componentType, patterns := range cd.namingPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(nameWithoutExt) {
				return &types.ComponentInfo{
					Type:        componentType,
					Name:        cd.generateComponentName(baseName, componentType),
					FilePath:    filePath,
					FileID:      fileID,
					Language:    language,
					Description: cd.generateDescription(componentType, baseName),
					Symbols:     symbols,
					Patterns:    []string{pattern.String()},
					Confidence:  cd.calculateNamingConfidence(componentType, nameWithoutExt),
					Evidence:    []string{"Filename matches pattern: " + pattern.String()},
					Metadata:    map[string]interface{}{"classification_method": "filename_pattern"},
				}
			}
		}
	}

	return nil
}

// classifyBySymbol classifies a component based on individual symbols
func (cd *ComponentDetector) classifyBySymbol(fileID types.FileID, filePath, language string, symbol types.Symbol) *types.ComponentInfo {
	// Check symbol-based rules
	for componentType, rules := range cd.symbolPatterns {
		for _, rule := range rules {
			if cd.symbolMatchesRule(symbol, rule) {
				return &types.ComponentInfo{
					Type:        componentType,
					Name:        symbol.Name,
					FilePath:    filePath,
					FileID:      fileID,
					Language:    language,
					Description: cd.generateSymbolDescription(componentType, symbol),
					Symbols:     []types.Symbol{symbol},
					Patterns:    []string{rule.NamePattern.String()},
					Confidence:  rule.Confidence,
					Evidence:    []string{fmt.Sprintf("Symbol %s matches %s pattern", symbol.Name, componentType.String())},
					Metadata: map[string]interface{}{
						"classification_method": "symbol_analysis",
						"symbol_type":           symbol.Type.String(),
						"symbol_line":           symbol.Line,
					},
				}
			}
		}
	}

	return nil
}

// Helper functions

func (cd *ComponentDetector) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	default:
		return "unknown"
	}
}

func (cd *ComponentDetector) isTestFile(filePath, language string) bool {
	patterns := globalPatterns.pathPatterns[types.ComponentTypeTest]
	for _, pattern := range patterns {
		if pattern.MatchString(filePath) {
			return true
		}
	}
	return false
}

func (cd *ComponentDetector) containsComponentType(types []types.ComponentType, target types.ComponentType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

func (cd *ComponentDetector) symbolMatchesRule(symbol types.Symbol, rule SymbolRule) bool {
	// Check symbol kind by comparing string representations
	if rule.SymbolKind.String() != symbol.Type.String() {
		return false
	}

	// Check name pattern
	if rule.NamePattern != nil && !rule.NamePattern.MatchString(symbol.Name) {
		return false
	}

	return true
}

func (cd *ComponentDetector) generateComponentName(baseName string, componentType types.ComponentType) string {
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	return fmt.Sprintf("%s (%s)", nameWithoutExt, componentType.String())
}

func (cd *ComponentDetector) generateDescription(componentType types.ComponentType, baseName string) string {
	switch componentType {
	case types.ComponentTypeEntryPoint:
		return "Entry point: " + baseName
	case types.ComponentTypeAPIHandler:
		return "API handler: " + baseName
	case types.ComponentTypeViewController:
		return "View component: " + baseName
	case types.ComponentTypeController:
		return "Controller: " + baseName
	case types.ComponentTypeDataModel:
		return "Data model: " + baseName
	case types.ComponentTypeConfiguration:
		return "Configuration: " + baseName
	case types.ComponentTypeTest:
		return "Test: " + baseName
	case types.ComponentTypeUtility:
		return "Utility: " + baseName
	case types.ComponentTypeService:
		return "Service: " + baseName
	case types.ComponentTypeRepository:
		return "Repository: " + baseName
	default:
		return "Component: " + baseName
	}
}

func (cd *ComponentDetector) generateSymbolDescription(componentType types.ComponentType, symbol types.Symbol) string {
	return fmt.Sprintf("%s: %s (%s at line %d)", componentType.String(), symbol.Name, symbol.Type.String(), symbol.Line)
}

func (cd *ComponentDetector) calculatePathConfidence(componentType types.ComponentType, filePath string) float64 {
	// Higher confidence for specific patterns
	if componentType == types.ComponentTypeEntryPoint && strings.Contains(filePath, "main.go") {
		return 0.95
	}
	if componentType == types.ComponentTypeTest && strings.Contains(filePath, "_test.go") {
		return 0.9
	}

	// Default confidence based on component type
	switch componentType {
	case types.ComponentTypeEntryPoint:
		return 0.85
	case types.ComponentTypeTest:
		return 0.8
	case types.ComponentTypeAPIHandler, types.ComponentTypeViewController:
		return 0.75
	default:
		return 0.7
	}
}

func (cd *ComponentDetector) calculateNamingConfidence(componentType types.ComponentType, name string) float64 {
	// Lower confidence than path-based classification
	return cd.calculatePathConfidence(componentType, "") - 0.1
}

func (cd *ComponentDetector) createGenericComponent(fileID types.FileID, filePath, language string, symbols []types.Symbol) *types.ComponentInfo {
	baseName := filepath.Base(filePath)

	// Try to infer type from symbols
	if len(symbols) > 0 {
		// If file has mainly structs/types, classify as data model
		structCount := 0
		for _, symbol := range symbols {
			if symbol.Type == types.SymbolTypeStruct || symbol.Type == types.SymbolTypeType || symbol.Type == types.SymbolTypeInterface {
				structCount++
			}
		}

		if float64(structCount)/float64(len(symbols)) > 0.5 {
			return &types.ComponentInfo{
				Type:        types.ComponentTypeDataModel,
				Name:        cd.generateComponentName(baseName, types.ComponentTypeDataModel),
				FilePath:    filePath,
				FileID:      fileID,
				Language:    language,
				Description: cd.generateDescription(types.ComponentTypeDataModel, baseName),
				Symbols:     symbols,
				Confidence:  0.4,
				Evidence:    []string{"File contains primarily data structures"},
				Metadata:    map[string]interface{}{"classification_method": "symbol_analysis_fallback"},
			}
		}
	}

	// Default to utility
	return &types.ComponentInfo{
		Type:        types.ComponentTypeUtility,
		Name:        cd.generateComponentName(baseName, types.ComponentTypeUtility),
		FilePath:    filePath,
		FileID:      fileID,
		Language:    language,
		Description: cd.generateDescription(types.ComponentTypeUtility, baseName),
		Symbols:     symbols,
		Confidence:  0.3,
		Evidence:    []string{"Default classification"},
		Metadata:    map[string]interface{}{"classification_method": "default_fallback"},
	}
}
