package symbollinker

import (
	"os"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	"github.com/standardbeagle/lci/internal/types"
)

// TestPHPExtractor tests the p h p extractor.
func TestPHPExtractor(t *testing.T) {
	// Read test file
	content, err := os.ReadFile("testdata/php_project/simple.php")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Create parser
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(tree_sitter_php.LanguagePHP())
	_ = parser.SetLanguage(lang)

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		t.Fatal("Failed to parse PHP code")
	}
	defer tree.Close()

	// Create extractor and extract symbols
	extractor := NewPHPExtractor()
	fileID := types.FileID(1)

	table, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	t.Run("Language verification", func(t *testing.T) {
		if table == nil {
			t.Fatal("Symbol table is nil")
		}

		if table.Language != "php" {
			t.Errorf("Expected language 'php', got %s", table.Language)
		}
	})

	t.Run("Namespace extraction", func(t *testing.T) {
		// Check for namespace declaration
		namespaceSymbols := findSymbolsByName(table, "App\\Examples")
		if len(namespaceSymbols) == 0 {
			t.Error("Namespace 'App\\Examples' not found")
		} else {
			symbol := namespaceSymbols[0]
			if symbol.Kind != types.SymbolKindNamespace {
				t.Errorf("Expected namespace, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Use statement extraction", func(t *testing.T) {
		expectedUses := map[string]string{
			"App\\Services\\UserService":           "App\\Services\\UserService",
			"App\\Models\\User":                    "App\\Models\\User",
			"App\\Interfaces\\ServiceInterface":    "App\\Interfaces\\ServiceInterface",
			"App\\Interfaces\\RepositoryInterface": "App\\Interfaces\\RepositoryInterface",
		}

		if len(table.Imports) < len(expectedUses) {
			t.Errorf("Expected at least %d imports, got %d", len(expectedUses), len(table.Imports))
		}

		foundUses := make(map[string]bool)
		for _, imp := range table.Imports {
			foundUses[imp.ImportPath] = true
		}

		for expectedUse := range expectedUses {
			if !foundUses[expectedUse] {
				t.Errorf("Use statement '%s' not found", expectedUse)
			}
		}
	})

	t.Run("Class extraction", func(t *testing.T) {
		// Check main class
		simpleClass := findSymbolsByName(table, "SimpleClass")
		if len(simpleClass) == 0 {
			t.Error("SimpleClass not found")
		} else {
			symbol := simpleClass[0]
			if symbol.Kind != types.SymbolKindClass {
				t.Errorf("Expected class, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("SimpleClass should be exported")
			}
		}

		// Check abstract class
		abstractBase := findSymbolsByName(table, "AbstractBase")
		if len(abstractBase) == 0 {
			t.Error("AbstractBase not found")
		} else {
			if abstractBase[0].Kind != types.SymbolKindClass {
				t.Errorf("Expected class, got %v", abstractBase[0].Kind)
			}
		}

		// Check final class
		finalClass := findSymbolsByName(table, "FinalClass")
		if len(finalClass) == 0 {
			t.Error("FinalClass not found")
		}
	})

	t.Run("Interface extraction", func(t *testing.T) {
		repositoryInterface := findSymbolsByName(table, "RepositoryInterface")
		if len(repositoryInterface) == 0 {
			t.Error("RepositoryInterface not found")
		} else {
			symbol := repositoryInterface[0]
			if symbol.Kind != types.SymbolKindInterface {
				t.Errorf("Expected interface, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Trait extraction", func(t *testing.T) {
		exampleTrait := findSymbolsByName(table, "ExampleTrait")
		if len(exampleTrait) == 0 {
			t.Error("ExampleTrait not found")
		} else {
			symbol := exampleTrait[0]
			if symbol.Kind != types.SymbolKindTrait {
				t.Errorf("Expected trait, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Property extraction", func(t *testing.T) {
		// Check class properties with visibility
		publicProp := findSymbolsByName(table, "SimpleClass.publicProperty")
		if len(publicProp) == 0 {
			t.Error("SimpleClass.publicProperty not found")
		} else {
			if publicProp[0].Kind != types.SymbolKindProperty {
				t.Errorf("Expected property, got %v", publicProp[0].Kind)
			}
			if !publicProp[0].IsExported {
				t.Error("publicProperty should be exported")
			}
		}

		privateProp := findSymbolsByName(table, "SimpleClass.privateProperty")
		if len(privateProp) == 0 {
			t.Error("SimpleClass.privateProperty not found")
		} else {
			if privateProp[0].IsExported {
				t.Error("privateProperty should not be exported")
			}
		}
	})

	t.Run("Method extraction", func(t *testing.T) {
		// Check public method
		publicMethod := findSymbolsByName(table, "SimpleClass.publicMethod")
		if len(publicMethod) == 0 {
			t.Error("SimpleClass.publicMethod not found")
		} else {
			symbol := publicMethod[0]
			if symbol.Kind != types.SymbolKindMethod {
				t.Errorf("Expected method, got %v", symbol.Kind)
			}
			if !symbol.IsExported {
				t.Error("publicMethod should be exported")
			}
			if symbol.Signature == "" {
				t.Error("publicMethod should have a signature")
			}
		}

		// Check private method
		privateMethod := findSymbolsByName(table, "SimpleClass.privateMethod")
		if len(privateMethod) == 0 {
			t.Error("SimpleClass.privateMethod not found")
		} else {
			if privateMethod[0].IsExported {
				t.Error("privateMethod should not be exported")
			}
		}

		// Check static method
		staticMethod := findSymbolsByName(table, "SimpleClass.staticMethod")
		if len(staticMethod) == 0 {
			t.Error("SimpleClass.staticMethod not found")
		}
	})

	t.Run("Constant extraction", func(t *testing.T) {
		// Check class constants
		publicConst := findSymbolsByName(table, "SimpleClass.PUBLIC_CONSTANT")
		if len(publicConst) == 0 {
			t.Error("SimpleClass.PUBLIC_CONSTANT not found")
		} else {
			if publicConst[0].Kind != types.SymbolKindConstant {
				t.Errorf("Expected constant, got %v", publicConst[0].Kind)
			}
			if !publicConst[0].IsExported {
				t.Error("PUBLIC_CONSTANT should be exported")
			}
		}

		privateConst := findSymbolsByName(table, "SimpleClass.PRIVATE_CONSTANT")
		if len(privateConst) == 0 {
			t.Error("SimpleClass.PRIVATE_CONSTANT not found")
		} else {
			if privateConst[0].IsExported {
				t.Error("PRIVATE_CONSTANT should not be exported")
			}
		}

		// Check global constants
		globalConst := findSymbolsByName(table, "DEFINED_CONSTANT")
		if len(globalConst) == 0 {
			t.Error("DEFINED_CONSTANT not found")
		}

		constConst := findSymbolsByName(table, "GLOBAL_CONST")
		if len(constConst) == 0 {
			t.Error("GLOBAL_CONST not found")
		}
	})

	t.Run("Function extraction", func(t *testing.T) {
		// Check global function
		globalFunc := findSymbolsByName(table, "global_function")
		if len(globalFunc) == 0 {
			t.Error("global_function not found")
		} else {
			symbol := globalFunc[0]
			if symbol.Kind != types.SymbolKindFunction {
				t.Errorf("Expected function, got %v", symbol.Kind)
			}
			if symbol.Signature == "" {
				t.Error("global_function should have a signature")
			}
		}

		// Check variadic function
		variadicFunc := findSymbolsByName(table, "variadic_function")
		if len(variadicFunc) == 0 {
			t.Error("variadic_function not found")
		}
	})

	t.Run("Enum extraction", func(t *testing.T) {
		// Check enum
		statusEnum := findSymbolsByName(table, "Status")
		if len(statusEnum) == 0 {
			t.Error("Status enum not found")
		} else {
			if statusEnum[0].Kind != types.SymbolKindEnum {
				t.Errorf("Expected enum, got %v", statusEnum[0].Kind)
			}
		}

		// Check enum cases
		pendingCase := findSymbolsByName(table, "Status.PENDING")
		if len(pendingCase) == 0 {
			t.Error("Status.PENDING not found")
		} else {
			if pendingCase[0].Kind != types.SymbolKindEnumMember {
				t.Errorf("Expected enum member, got %v", pendingCase[0].Kind)
			}
		}
	})

	t.Run("Variable extraction", func(t *testing.T) {
		// Check global variables
		globalVar := findSymbolsByName(table, "globalVar")
		if len(globalVar) == 0 {
			t.Error("globalVar not found")
		} else {
			if globalVar[0].Kind != types.SymbolKindVariable {
				t.Errorf("Expected variable, got %v", globalVar[0].Kind)
			}
		}
	})

	t.Run("Symbol count", func(t *testing.T) {
		// Ensure we're extracting a reasonable number of symbols
		if len(table.Symbols) < 40 {
			t.Errorf("Expected at least 40 symbols, got %d", len(table.Symbols))
		}

		t.Logf("Total PHP symbols extracted: %d", len(table.Symbols))

		// Log symbol distribution by kind
		kindCounts := make(map[types.SymbolKind]int)
		for _, sym := range table.Symbols {
			kindCounts[sym.Kind]++
		}

		for kind, count := range kindCounts {
			t.Logf("  %s: %d", kind.String(), count)
		}
	})
}

// TestPHPExtractorCanHandle tests the p h p extractor can handle.
func TestPHPExtractorCanHandle(t *testing.T) {
	extractor := NewPHPExtractor()

	tests := []struct {
		filepath string
		expected bool
	}{
		{"index.php", true},
		{"class.php", true},
		{"file.PHP", false}, // Case sensitive
		{"script.phtml", true},
		{"legacy.php3", true},
		{"archive.phar", true},
		{"test.js", false},
		{"test.py", false},
		{"/path/to/file.php", true},
		{"", false},
	}

	for _, test := range tests {
		if extractor.CanHandle(test.filepath) != test.expected {
			t.Errorf("CanHandle(%q) = %v, expected %v",
				test.filepath, !test.expected, test.expected)
		}
	}
}

// TestPHPExtractorLanguage tests the p h p extractor language.
func TestPHPExtractorLanguage(t *testing.T) {
	extractor := NewPHPExtractor()

	if extractor.GetLanguage() != "php" {
		t.Errorf("Expected language 'php', got %s", extractor.GetLanguage())
	}
}

// TestPHPExtractorPHP8Features tests PHP 8.0+ feature extraction
func TestPHPExtractorPHP8Features(t *testing.T) {
	// Read test file with modern PHP 8.0+ features
	content, err := os.ReadFile("testdata/php_project/modern_php8.php")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Create parser
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(tree_sitter_php.LanguagePHP())
	_ = parser.SetLanguage(lang)

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		t.Fatal("Failed to parse PHP 8.0+ code")
	}
	defer tree.Close()

	// Create extractor and extract symbols
	extractor := NewPHPExtractor()
	fileID := types.FileID(1)

	table, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	t.Run("Class with attributes extraction", func(t *testing.T) {
		// Check class with attributes
		userController := findSymbolsByName(table, "UserController")
		if len(userController) == 0 {
			t.Error("UserController not found")
		} else {
			symbol := userController[0]
			if symbol.Kind != types.SymbolKindClass {
				t.Errorf("Expected class, got %v", symbol.Kind)
			}
			// Check that attributes were captured in signature
			if symbol.Signature != "" && !contains(symbol.Signature, "Route") {
				t.Logf("Class signature: %s", symbol.Signature)
			}
		}
	})

	t.Run("Constructor property promotion", func(t *testing.T) {
		// Check promoted properties - they should exist as properties with readonly/visibility in signature
		// The promoted properties are stored with $ prefix like "$name"
		foundReadonlyProperty := false
		foundProtectedProperty := false

		for _, sym := range table.Symbols {
			if sym.Kind == types.SymbolKindProperty {
				// Check for readonly property (private readonly string $name)
				if contains(sym.Signature, "readonly") && contains(sym.Signature, "string") {
					foundReadonlyProperty = true
					t.Logf("Found readonly promoted property: %s with signature: %s", sym.Name, sym.Signature)
				}
				// Check for protected property (protected int $id)
				if contains(sym.Signature, "protected") && contains(sym.Signature, "int") {
					foundProtectedProperty = true
					t.Logf("Found protected promoted property: %s with signature: %s", sym.Name, sym.Signature)
				}
			}
		}

		if !foundReadonlyProperty {
			t.Error("No readonly promoted property found (expected private readonly string)")
		}
		if !foundProtectedProperty {
			t.Error("No protected promoted property found (expected protected int)")
		}
	})

	t.Run("Method with attributes extraction", func(t *testing.T) {
		// Check method with attributes
		indexMethod := findSymbolsByName(table, "UserController.index")
		if len(indexMethod) == 0 {
			t.Error("UserController.index method not found")
		} else {
			symbol := indexMethod[0]
			if symbol.Kind != types.SymbolKindMethod {
				t.Errorf("Expected method, got %v", symbol.Kind)
			}
		}

		// Check show method
		showMethod := findSymbolsByName(table, "UserController.show")
		if len(showMethod) == 0 {
			t.Error("UserController.show method not found")
		}
	})

	t.Run("Enum with attribute extraction", func(t *testing.T) {
		// Check enum
		userStatusEnum := findSymbolsByName(table, "UserStatus")
		if len(userStatusEnum) == 0 {
			t.Error("UserStatus enum not found")
		} else {
			symbol := userStatusEnum[0]
			if symbol.Kind != types.SymbolKindEnum {
				t.Errorf("Expected enum, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Readonly class extraction", func(t *testing.T) {
		// Check readonly DTO class
		userDTO := findSymbolsByName(table, "UserDTO")
		if len(userDTO) == 0 {
			t.Error("UserDTO not found")
		} else {
			symbol := userDTO[0]
			if symbol.Kind != types.SymbolKindClass {
				t.Errorf("Expected class, got %v", symbol.Kind)
			}
		}
	})

	t.Run("Symbol count for modern PHP", func(t *testing.T) {
		t.Logf("Total PHP 8.0+ symbols extracted: %d", len(table.Symbols))

		// Log all symbols for debugging
		for _, sym := range table.Symbols {
			t.Logf("  %s: %s (sig: %s)", sym.Kind.String(), sym.Name, sym.Signature)
		}
	})
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestPHPExtractorWordPressHooks tests WordPress hook detection
func TestPHPExtractorWordPressHooks(t *testing.T) {
	// Read test file with WordPress hooks
	content, err := os.ReadFile("testdata/php_project/wordpress_hooks.php")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Create parser
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(tree_sitter_php.LanguagePHP())
	_ = parser.SetLanguage(lang)

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		t.Fatal("Failed to parse WordPress PHP code")
	}
	defer tree.Close()

	// Create extractor and extract symbols
	extractor := NewPHPExtractor()
	fileID := types.FileID(1)

	table, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	t.Run("Action hook extraction", func(t *testing.T) {
		// Check for add_action hooks
		initHook := findSymbolsByName(table, "hook:action:init")
		if len(initHook) == 0 {
			t.Error("hook:action:init not found")
		} else {
			symbol := initHook[0]
			if symbol.Kind != types.SymbolKindEvent {
				t.Errorf("Expected event kind, got %v", symbol.Kind)
			}
			if symbol.Type != "action" {
				t.Errorf("Expected type 'action', got %s", symbol.Type)
			}
		}

		// Check action with priority
		wpHeadHook := findSymbolsByName(table, "hook:action:wp_head")
		if len(wpHeadHook) == 0 {
			t.Error("hook:action:wp_head not found")
		}

		// Check action with array callback
		pluginsLoadedHook := findSymbolsByName(table, "hook:action:plugins_loaded")
		if len(pluginsLoadedHook) == 0 {
			t.Error("hook:action:plugins_loaded not found")
		}
	})

	t.Run("Filter hook extraction", func(t *testing.T) {
		// Check for add_filter hooks
		contentHook := findSymbolsByName(table, "hook:filter:the_content")
		if len(contentHook) == 0 {
			t.Error("hook:filter:the_content not found")
		} else {
			symbol := contentHook[0]
			if symbol.Type != "filter" {
				t.Errorf("Expected type 'filter', got %s", symbol.Type)
			}
		}

		// Check filter with priority
		excerptHook := findSymbolsByName(table, "hook:filter:excerpt_length")
		if len(excerptHook) == 0 {
			t.Error("hook:filter:excerpt_length not found")
		}
	})

	t.Run("Shortcode extraction", func(t *testing.T) {
		// Check for shortcode registrations
		buttonShortcode := findSymbolsByName(table, "hook:shortcode:my_button")
		if len(buttonShortcode) == 0 {
			t.Error("hook:shortcode:my_button not found")
		} else {
			symbol := buttonShortcode[0]
			if symbol.Type != "shortcode" {
				t.Errorf("Expected type 'shortcode', got %s", symbol.Type)
			}
		}

		contactFormShortcode := findSymbolsByName(table, "hook:shortcode:contact_form")
		if len(contactFormShortcode) == 0 {
			t.Error("hook:shortcode:contact_form not found")
		}
	})

	t.Run("REST API route extraction", func(t *testing.T) {
		// Check for REST route registrations
		// Note: Hook name combines namespace and path
		restRoutes := findPHPSymbolsByPrefix(table, "hook:rest_route:")
		if len(restRoutes) == 0 {
			t.Error("No REST route hooks found")
		} else {
			t.Logf("Found %d REST route hooks", len(restRoutes))
			for _, route := range restRoutes {
				t.Logf("  REST route: %s (sig: %s)", route.Name, route.Signature)
			}
		}
	})

	t.Run("Hook signature verification", func(t *testing.T) {
		// Verify signatures contain proper callback info
		initHook := findSymbolsByName(table, "hook:action:init")
		if len(initHook) > 0 {
			sig := initHook[0].Signature
			if !contains(sig, "add_action") {
				t.Errorf("Expected signature to contain 'add_action', got: %s", sig)
			}
			if !contains(sig, "init") {
				t.Errorf("Expected signature to contain 'init', got: %s", sig)
			}
		}
	})

	t.Run("Class method hooks extraction", func(t *testing.T) {
		// Check hooks registered inside class constructor
		classInitHook := findPHPSymbolsContaining(table, "hook:action:init")
		// There should be multiple 'init' hooks (one global, one in class)
		if len(classInitHook) < 2 {
			t.Logf("Expected at least 2 init hooks, found %d", len(classInitHook))
		}
	})

	t.Run("WordPress hook count", func(t *testing.T) {
		// Count all hook symbols
		hookCount := 0
		for _, sym := range table.Symbols {
			if contains(sym.Name, "hook:") {
				hookCount++
			}
		}

		// We expect at least 10 hooks from the test file
		if hookCount < 10 {
			t.Errorf("Expected at least 10 hook symbols, got %d", hookCount)
		}

		t.Logf("Total WordPress hooks extracted: %d", hookCount)

		// Log all hooks for debugging
		for _, sym := range table.Symbols {
			if contains(sym.Name, "hook:") {
				t.Logf("  %s: %s (type: %s, sig: %s)", sym.Kind.String(), sym.Name, sym.Type, sym.Signature)
			}
		}
	})
}

// findPHPSymbolsByPrefix finds symbols whose name starts with a prefix
func findPHPSymbolsByPrefix(table *types.SymbolTable, prefix string) []*types.EnhancedSymbolInfo {
	var symbols []*types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if len(sym.Name) >= len(prefix) && sym.Name[:len(prefix)] == prefix {
			symbols = append(symbols, sym)
		}
	}
	return symbols
}

// findPHPSymbolsContaining finds symbols whose name contains a substring
func findPHPSymbolsContaining(table *types.SymbolTable, substr string) []*types.EnhancedSymbolInfo {
	var symbols []*types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if contains(sym.Name, substr) {
			symbols = append(symbols, sym)
		}
	}
	return symbols
}

// TestPHPExtractorWordPressAdvanced tests WordPress plugin metadata, templates, and Gutenberg blocks
func TestPHPExtractorWordPressAdvanced(t *testing.T) {
	// Read test file with advanced WordPress patterns
	content, err := os.ReadFile("testdata/php_project/wordpress_advanced.php")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Create parser
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(tree_sitter_php.LanguagePHP())
	_ = parser.SetLanguage(lang)

	// Parse the content
	tree := parser.Parse(content, nil)
	if tree == nil {
		t.Fatal("Failed to parse WordPress PHP code")
	}
	defer tree.Close()

	// Create extractor and extract symbols
	extractor := NewPHPExtractor()
	fileID := types.FileID(1)

	table, err := extractor.ExtractSymbols(fileID, content, tree)
	if err != nil {
		t.Fatalf("Failed to extract symbols: %v", err)
	}

	t.Run("Plugin metadata extraction", func(t *testing.T) {
		// Check for plugin metadata
		pluginMeta := findSymbolsByName(table, "wp:plugin:My Awesome Plugin")
		if len(pluginMeta) == 0 {
			t.Error("wp:plugin:My Awesome Plugin not found")
		} else {
			symbol := pluginMeta[0]
			if symbol.Kind != types.SymbolKindModule {
				t.Errorf("Expected module kind, got %v", symbol.Kind)
			}
			if symbol.Type != "plugin" {
				t.Errorf("Expected type 'plugin', got %s", symbol.Type)
			}
			if symbol.Value != "2.1.0" {
				t.Errorf("Expected version '2.1.0', got %s", symbol.Value)
			}
			// Check signature contains description and author
			if !contains(symbol.Signature, "John Doe") {
				t.Errorf("Expected signature to contain author, got: %s", symbol.Signature)
			}
			t.Logf("Plugin metadata: %s (version: %s, sig: %s)", symbol.Name, symbol.Value, symbol.Signature)
		}
	})

	t.Run("Template metadata extraction", func(t *testing.T) {
		// Check for template metadata - Full Width Page
		fullWidthTemplate := findSymbolsByName(table, "wp:template:Full Width Page")
		if len(fullWidthTemplate) == 0 {
			t.Error("wp:template:Full Width Page not found")
		} else {
			symbol := fullWidthTemplate[0]
			if symbol.Type != "template" {
				t.Errorf("Expected type 'template', got %s", symbol.Type)
			}
			// Check signature contains post types
			if !contains(symbol.Signature, "page") {
				t.Errorf("Expected signature to contain post types, got: %s", symbol.Signature)
			}
			t.Logf("Template: %s (sig: %s)", symbol.Name, symbol.Signature)
		}

		// Check for sidebar template
		sidebarTemplate := findSymbolsByName(table, "wp:template:Sidebar Layout")
		if len(sidebarTemplate) == 0 {
			t.Error("wp:template:Sidebar Layout not found")
		}
	})

	t.Run("Gutenberg block extraction", func(t *testing.T) {
		// Check for hero block (string name)
		heroBlock := findSymbolsByName(table, "block:myplugin/hero-block")
		if len(heroBlock) == 0 {
			t.Error("block:myplugin/hero-block not found")
		} else {
			symbol := heroBlock[0]
			if symbol.Type != "gutenberg_block" {
				t.Errorf("Expected type 'gutenberg_block', got %s", symbol.Type)
			}
			if !contains(symbol.Signature, "render_hero_block") {
				t.Errorf("Expected signature to contain render callback, got: %s", symbol.Signature)
			}
			t.Logf("Block: %s (sig: %s)", symbol.Name, symbol.Signature)
		}

		// Check for gallery block (path-based)
		galleryBlocks := findPHPSymbolsByPrefix(table, "block:__DIR__")
		if len(galleryBlocks) == 0 {
			t.Error("Path-based blocks not found")
		} else {
			t.Logf("Found %d path-based blocks", len(galleryBlocks))
			for _, block := range galleryBlocks {
				t.Logf("  Path block: %s", block.Name)
			}
		}

		// Check for custom CTA block (WP_Block_Type)
		ctaBlock := findSymbolsByName(table, "block:myplugin/custom-cta")
		if len(ctaBlock) == 0 {
			t.Error("block:myplugin/custom-cta not found")
		} else {
			symbol := ctaBlock[0]
			if !contains(symbol.Signature, "render_cta_block") {
				t.Errorf("Expected signature to contain render callback, got: %s", symbol.Signature)
			}
			t.Logf("CTA Block: %s (sig: %s)", symbol.Name, symbol.Signature)
		}
	})

	t.Run("WordPress patterns count", func(t *testing.T) {
		// Count plugin/theme/template symbols
		wpMetaCount := 0
		blockCount := 0
		hookCount := 0

		for _, sym := range table.Symbols {
			if contains(sym.Name, "wp:") {
				wpMetaCount++
			}
			if contains(sym.Name, "block:") {
				blockCount++
			}
			if contains(sym.Name, "hook:") {
				hookCount++
			}
		}

		t.Logf("WordPress metadata symbols: %d", wpMetaCount)
		t.Logf("Gutenberg block symbols: %d", blockCount)
		t.Logf("Hook symbols: %d", hookCount)

		// We expect at least 1 plugin, 2 templates
		if wpMetaCount < 3 {
			t.Errorf("Expected at least 3 WordPress metadata symbols, got %d", wpMetaCount)
		}

		// We expect at least 4 blocks (hero, gallery, testimonial, cta)
		if blockCount < 4 {
			t.Errorf("Expected at least 4 block symbols, got %d", blockCount)
		}

		// Log all WordPress-related symbols for debugging
		t.Log("All WordPress patterns found:")
		for _, sym := range table.Symbols {
			if contains(sym.Name, "wp:") || contains(sym.Name, "block:") {
				t.Logf("  %s: %s (type: %s, sig: %s)", sym.Kind.String(), sym.Name, sym.Type, sym.Signature)
			}
		}
	})
}
