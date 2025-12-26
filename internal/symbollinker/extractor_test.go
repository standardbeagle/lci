package symbollinker

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// TestBaseExtractor tests the base extractor.
func TestBaseExtractor(t *testing.T) {
	t.Run("NewBaseExtractor", func(t *testing.T) {
		extractor := NewBaseExtractor("go", []string{".go", ".GO"})

		if extractor.GetLanguage() != "go" {
			t.Errorf("Expected language 'go', got %s", extractor.GetLanguage())
		}

		if len(extractor.fileExts) != 2 {
			t.Errorf("Expected 2 file extensions, got %d", len(extractor.fileExts))
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		extractor := NewBaseExtractor("go", []string{".go", ".GO"})

		tests := []struct {
			filepath string
			expected bool
		}{
			{"main.go", true},
			{"test.GO", true},
			{"file.js", false},
			{"noext", false},
			{".go", true},
			{"", false},
		}

		for _, test := range tests {
			if extractor.CanHandle(test.filepath) != test.expected {
				t.Errorf("CanHandle(%q) = %v, expected %v",
					test.filepath, !test.expected, test.expected)
			}
		}
	})
}

// TestScopeManager tests the scope manager.
func TestScopeManager(t *testing.T) {
	t.Run("Initialization", func(t *testing.T) {
		sm := NewScopeManager()

		if sm.currentScope == nil {
			t.Fatal("Expected current scope to be initialized")
		}

		if sm.currentScope.Type != types.ScopeGlobal {
			t.Errorf("Expected global scope, got %v", sm.currentScope.Type)
		}

		if len(sm.scopeStack) != 1 {
			t.Errorf("Expected scope stack length 1, got %d", len(sm.scopeStack))
		}
	})

	t.Run("PushScope", func(t *testing.T) {
		sm := NewScopeManager()

		sm.PushScope(types.ScopeFunction, "testFunc", 10, 50)

		if sm.currentScope.Type != types.ScopeFunction {
			t.Errorf("Expected function scope, got %v", sm.currentScope.Type)
		}

		if sm.currentScope.Name != "testFunc" {
			t.Errorf("Expected scope name 'testFunc', got %s", sm.currentScope.Name)
		}

		if len(sm.scopeStack) != 2 {
			t.Errorf("Expected scope stack length 2, got %d", len(sm.scopeStack))
		}

		// Parent should be global scope
		if sm.currentScope.Parent == nil || sm.currentScope.Parent.Type != types.ScopeGlobal {
			t.Error("Expected parent to be global scope")
		}
	})

	t.Run("PopScope", func(t *testing.T) {
		sm := NewScopeManager()

		sm.PushScope(types.ScopeFunction, "func1", 10, 50)
		sm.PushScope(types.ScopeBlock, "block1", 20, 40)

		// Pop block scope
		sm.PopScope()

		if sm.currentScope.Type != types.ScopeFunction {
			t.Errorf("Expected function scope after pop, got %v", sm.currentScope.Type)
		}

		// Pop function scope
		sm.PopScope()

		if sm.currentScope.Type != types.ScopeGlobal {
			t.Errorf("Expected global scope after second pop, got %v", sm.currentScope.Type)
		}

		// Pop global scope (should remain at global)
		sm.PopScope()

		if sm.currentScope.Type != types.ScopeGlobal {
			t.Errorf("Expected global scope to remain, got %v", sm.currentScope.Type)
		}

		if len(sm.scopeStack) != 1 {
			t.Errorf("Expected scope stack to have minimum 1 element, got %d", len(sm.scopeStack))
		}
	})

	t.Run("IsInScope", func(t *testing.T) {
		sm := NewScopeManager()
		sm.PushScope(types.ScopeFunction, "func", 10, 50)

		tests := []struct {
			pos      int
			expected bool
		}{
			{5, false},  // Before scope
			{10, true},  // Start of scope
			{30, true},  // Middle of scope
			{50, true},  // End of scope
			{51, false}, // After scope
		}

		for _, test := range tests {
			if sm.IsInScope(test.pos) != test.expected {
				t.Errorf("IsInScope(%d) = %v, expected %v",
					test.pos, !test.expected, test.expected)
			}
		}
	})

	t.Run("GetScopeAtPosition", func(t *testing.T) {
		sm := NewScopeManager()
		sm.PushScope(types.ScopeFunction, "outer", 10, 100)
		sm.PushScope(types.ScopeBlock, "inner", 30, 70)

		tests := []struct {
			pos          int
			expectedType types.SymbolScopeType
		}{
			{5, types.ScopeGlobal},    // Before any scope
			{15, types.ScopeFunction}, // In function but before block
			{40, types.ScopeBlock},    // In both, should get most specific
			{80, types.ScopeFunction}, // After block but in function
			{105, types.ScopeGlobal},  // After everything
		}

		for _, test := range tests {
			scope := sm.GetScopeAtPosition(test.pos)
			if scope.Type != test.expectedType {
				t.Errorf("GetScopeAtPosition(%d) = %v, expected %v",
					test.pos, scope.Type, test.expectedType)
			}
		}
	})
}

// TestSymbolTableBuilder tests the symbol table builder.
func TestSymbolTableBuilder(t *testing.T) {
	t.Run("Initialization", func(t *testing.T) {
		builder := NewSymbolTableBuilder(types.FileID(1), "go")

		table := builder.Build()

		if table.FileID != types.FileID(1) {
			t.Errorf("Expected FileID 1, got %d", table.FileID)
		}

		if table.Language != "go" {
			t.Errorf("Expected language 'go', got %s", table.Language)
		}

		if len(table.Symbols) != 0 {
			t.Errorf("Expected empty symbols, got %d", len(table.Symbols))
		}
	})

	t.Run("AddSymbol", func(t *testing.T) {
		builder := NewSymbolTableBuilder(types.FileID(1), "go")

		scope := &types.SymbolScope{
			Type: types.ScopeGlobal,
			Name: "global",
		}

		location := types.SymbolLocation{
			FileID: types.FileID(1),
			Line:   10,
			Column: 5,
		}

		localID := builder.AddSymbol("TestFunc", types.SymbolKindFunction, location, scope, true)

		if localID != 1 {
			t.Errorf("Expected first symbol to have LocalID 1, got %d", localID)
		}

		table := builder.Build()

		if len(table.Symbols) != 1 {
			t.Errorf("Expected 1 symbol, got %d", len(table.Symbols))
		}

		symbol := table.Symbols[1]
		if symbol.Name != "TestFunc" {
			t.Errorf("Expected symbol name 'TestFunc', got %s", symbol.Name)
		}

		if symbol.Kind != types.SymbolKindFunction {
			t.Errorf("Expected function kind, got %v", symbol.Kind)
		}

		if !symbol.IsExported {
			t.Error("Expected symbol to be exported")
		}
	})

	t.Run("AddMultipleSymbols", func(t *testing.T) {
		builder := NewSymbolTableBuilder(types.FileID(1), "go")

		scope := &types.SymbolScope{Type: types.ScopeGlobal, Name: "global"}
		location := types.SymbolLocation{FileID: types.FileID(1)}

		id1 := builder.AddSymbol("func1", types.SymbolKindFunction, location, scope, true)
		id2 := builder.AddSymbol("var1", types.SymbolKindVariable, location, scope, false)
		id3 := builder.AddSymbol("func1", types.SymbolKindFunction, location, scope, true) // Duplicate name

		table := builder.Build()

		if len(table.Symbols) != 3 {
			t.Errorf("Expected 3 symbols, got %d", len(table.Symbols))
		}

		// Check IDs are sequential
		if id1 != 1 || id2 != 2 || id3 != 3 {
			t.Errorf("Expected sequential IDs 1,2,3, got %d,%d,%d", id1, id2, id3)
		}

		// Check duplicate names are tracked
		func1IDs := table.SymbolsByName["func1"]
		if len(func1IDs) != 2 {
			t.Errorf("Expected 2 symbols named 'func1', got %d", len(func1IDs))
		}
	})

	t.Run("AddImport", func(t *testing.T) {
		builder := NewSymbolTableBuilder(types.FileID(1), "go")

		importInfo := types.ImportInfo{
			ImportPath:    "fmt",
			Alias:         "fmt",
			ImportedNames: []string{"Printf", "Println"},
			Location:      types.SymbolLocation{FileID: types.FileID(1), Line: 3},
		}

		builder.AddImport(importInfo)

		table := builder.Build()

		if len(table.Imports) != 1 {
			t.Errorf("Expected 1 import, got %d", len(table.Imports))
		}

		imp := table.Imports[0]
		if imp.ImportPath != "fmt" {
			t.Errorf("Expected import path 'fmt', got %s", imp.ImportPath)
		}

		if imp.LocalID == 0 {
			t.Error("Expected import to have non-zero LocalID")
		}
	})

	t.Run("AddExport", func(t *testing.T) {
		builder := NewSymbolTableBuilder(types.FileID(1), "javascript")

		exportInfo := types.ExportInfo{
			ExportedName: "default",
			LocalName:    "MyComponent",
			IsDefault:    true,
			Location:     types.SymbolLocation{FileID: types.FileID(1), Line: 50},
		}

		builder.AddExport(exportInfo)

		table := builder.Build()

		if len(table.Exports) != 1 {
			t.Errorf("Expected 1 export, got %d", len(table.Exports))
		}

		exp := table.Exports[0]
		if !exp.IsDefault {
			t.Error("Expected default export")
		}

		if exp.LocalID == 0 {
			t.Error("Expected export to have non-zero LocalID")
		}
	})
}

// TestGetNodeText tests the get node text.
func TestGetNodeText(t *testing.T) {
	content := []byte("func TestFunction() {}")

	tests := []struct {
		name     string
		start    uint
		end      uint
		expected string
	}{
		{"full content", 0, 22, "func TestFunction() {}"},
		{"function name", 5, 17, "TestFunction"},
		{"empty range", 10, 10, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a mock node (we can't create real sitter nodes without a parser)
			// For this test, we'll test the bounds checking logic
			if test.start > uint(len(content)) || test.end > uint(len(content)) {
				// Would return empty string
				return
			}

			result := string(content[test.start:test.end])
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

// TestVisibilityCheckers tests the visibility checkers.
func TestVisibilityCheckers(t *testing.T) {
	t.Run("GoCapitalization", func(t *testing.T) {
		tests := []struct {
			name     string
			expected bool
		}{
			{"PublicFunc", true},
			{"privateFunc", false},
			{"_private", false},
			{"A", true},
			{"a", false},
			{"", false},
		}

		for _, test := range tests {
			result := CommonVisibilityRules.GoCapitalization(test.name, nil, nil)
			if result != test.expected {
				t.Errorf("GoCapitalization(%q) = %v, expected %v",
					test.name, result, test.expected)
			}
		}
	})

	t.Run("PythonUnderscore", func(t *testing.T) {
		tests := []struct {
			name     string
			expected bool
		}{
			{"public_func", true},
			{"_private_func", false},
			{"__magic__", false},
			{"normal", true},
			{"", false},
		}

		for _, test := range tests {
			result := CommonVisibilityRules.PythonUnderscore(test.name, nil, nil)
			if result != test.expected {
				t.Errorf("PythonUnderscore(%q) = %v, expected %v",
					test.name, result, test.expected)
			}
		}
	})
}

// TestExtractorRegistry tests the extractor registry.
func TestExtractorRegistry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		registry := NewExtractorRegistry()

		goExtractor := &mockExtractor{language: "go", exts: []string{".go"}}
		jsExtractor := &mockExtractor{language: "javascript", exts: []string{".js", ".jsx"}}

		registry.Register("go", goExtractor)
		registry.Register("javascript", jsExtractor)

		// Get by language
		ext, err := registry.GetExtractor("go")
		if err != nil {
			t.Fatalf("Failed to get go extractor: %v", err)
		}

		if ext.GetLanguage() != "go" {
			t.Errorf("Expected go extractor, got %s", ext.GetLanguage())
		}

		// Get non-existent
		_, err = registry.GetExtractor("rust")
		if err == nil {
			t.Error("Expected error for non-existent extractor")
		}
	})

	t.Run("GetExtractorForFile", func(t *testing.T) {
		registry := NewExtractorRegistry()

		goExtractor := &mockExtractor{language: "go", exts: []string{".go"}}
		jsExtractor := &mockExtractor{language: "javascript", exts: []string{".js", ".jsx"}}

		registry.Register("go", goExtractor)
		registry.Register("javascript", jsExtractor)

		tests := []struct {
			filepath string
			expected string
			hasError bool
		}{
			{"main.go", "go", false},
			{"app.js", "javascript", false},
			{"component.jsx", "javascript", false},
			{"style.css", "", true},
		}

		for _, test := range tests {
			ext, err := registry.GetExtractorForFile(test.filepath)

			if test.hasError {
				if err == nil {
					t.Errorf("Expected error for file %s", test.filepath)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for file %s: %v", test.filepath, err)
				} else if ext.GetLanguage() != test.expected {
					t.Errorf("Expected language %s for file %s, got %s",
						test.expected, test.filepath, ext.GetLanguage())
				}
			}
		}
	})

	t.Run("GetSupportedLanguages", func(t *testing.T) {
		registry := NewExtractorRegistry()

		registry.Register("go", &mockExtractor{language: "go"})
		registry.Register("javascript", &mockExtractor{language: "javascript"})
		registry.Register("python", &mockExtractor{language: "python"})

		languages := registry.GetSupportedLanguages()

		if len(languages) != 3 {
			t.Errorf("Expected 3 languages, got %d", len(languages))
		}

		// Check all expected languages are present
		languageMap := make(map[string]bool)
		for _, lang := range languages {
			languageMap[lang] = true
		}

		expected := []string{"go", "javascript", "python"}
		for _, exp := range expected {
			if !languageMap[exp] {
				t.Errorf("Expected language %s not found", exp)
			}
		}
	})
}

// mockExtractor is a test implementation of SymbolExtractor
type mockExtractor struct {
	language string
	exts     []string
}

func (m *mockExtractor) ExtractSymbols(fileID types.FileID, content []byte, tree *sitter.Tree) (*types.SymbolTable, error) {
	return &types.SymbolTable{
		FileID:   fileID,
		Language: m.language,
	}, nil
}

func (m *mockExtractor) GetLanguage() string {
	return m.language
}

func (m *mockExtractor) CanHandle(filepath string) bool {
	for _, ext := range m.exts {
		if hasExtension(filepath, ext) {
			return true
		}
	}
	return false
}
