package parser

import (
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Phase 5C: Community Parser Framework for non-standard tree-sitter parsers
// This framework provides a unified interface for community-maintained parsers
// that don't follow the standard go-tree-sitter organization structure

// CommunityParserAdapter provides a standardized interface for community parsers
type CommunityParserAdapter struct {
	name        string
	extensions  []string
	getLanguage func() *tree_sitter.Language
	queryDef    string
}

// NewCommunityParserAdapter creates a new adapter for community parsers
func NewCommunityParserAdapter(name string, extensions []string, getLanguage func() *tree_sitter.Language, queryDef string) *CommunityParserAdapter {
	return &CommunityParserAdapter{
		name:        name,
		extensions:  extensions,
		getLanguage: getLanguage,
		queryDef:    queryDef,
	}
}

// Name returns the parser name
func (cpa *CommunityParserAdapter) Name() string {
	return cpa.name
}

// Extensions returns supported file extensions
func (cpa *CommunityParserAdapter) Extensions() []string {
	return cpa.extensions
}

// GetLanguage returns the tree-sitter language
func (cpa *CommunityParserAdapter) GetLanguage() *tree_sitter.Language {
	return cpa.getLanguage()
}

// GetQueryDefinition returns the tree-sitter query string
func (cpa *CommunityParserAdapter) GetQueryDefinition() string {
	return cpa.queryDef
}

// SetupParser configures a parser instance with this community parser
func (cpa *CommunityParserAdapter) SetupParser(p *TreeSitterParser) error {
	// Create parser instance
	parser := tree_sitter.NewParser()
	_ = parser.SetLanguage(cpa.getLanguage())

	// Register parser for all extensions
	for _, ext := range cpa.extensions {
		p.parsers[ext] = parser
	}

	// Create and register query
	query, err := tree_sitter.NewQuery(cpa.getLanguage(), cpa.queryDef)
	if err != nil {
		return fmt.Errorf("community parser query error for %s: %w", cpa.name, err)
	}
	// Note: query is stored in parser and will be closed when parser is destroyed

	// Register query for all extensions
	for _, ext := range cpa.extensions {
		p.queries[ext] = query
	}

	return nil
}

// CommunityParserRegistry manages all community parser adapters
type CommunityParserRegistry struct {
	adapters map[string]*CommunityParserAdapter
}

// NewCommunityParserRegistry creates a new registry
func NewCommunityParserRegistry() *CommunityParserRegistry {
	return &CommunityParserRegistry{
		adapters: make(map[string]*CommunityParserAdapter),
	}
}

// Register adds a community parser adapter to the registry
func (cpr *CommunityParserRegistry) Register(adapter *CommunityParserAdapter) {
	cpr.adapters[adapter.Name()] = adapter
}

// GetAdapter retrieves a community parser adapter by name
func (cpr *CommunityParserRegistry) GetAdapter(name string) (*CommunityParserAdapter, bool) {
	adapter, exists := cpr.adapters[name]
	return adapter, exists
}

// GetAdapterForExtension finds the appropriate adapter for a file extension
func (cpr *CommunityParserRegistry) GetAdapterForExtension(ext string) (*CommunityParserAdapter, bool) {
	for _, adapter := range cpr.adapters {
		for _, adapterExt := range adapter.Extensions() {
			if adapterExt == ext {
				return adapter, true
			}
		}
	}
	return nil, false
}

// SetupAllParsers configures all registered community parsers on a TreeSitterParser instance
func (cpr *CommunityParserRegistry) SetupAllParsers(p *TreeSitterParser) []error {
	var errors []error

	for name, adapter := range cpr.adapters {
		if err := adapter.SetupParser(p); err != nil {
			errors = append(errors, fmt.Errorf("failed to setup community parser %s: %w", name, err))
		}
	}

	return errors
}

// ListAdapters returns all registered adapter names
func (cpr *CommunityParserRegistry) ListAdapters() []string {
	names := make([]string, 0, len(cpr.adapters))
	for name := range cpr.adapters {
		names = append(names, name)
	}
	return names
}
