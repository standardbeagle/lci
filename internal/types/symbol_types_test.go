package types

import (
	"encoding/json"
	"testing"
)

// TestCompositeSymbolID tests the composite symbol i d.
func TestCompositeSymbolID(t *testing.T) {
	t.Run("NewCompositeSymbolID", func(t *testing.T) {
		fileID := FileID(42)
		localID := uint32(123)

		sym := NewCompositeSymbolID(fileID, localID)

		if sym.FileID != fileID {
			t.Errorf("Expected FileID %d, got %d", fileID, sym.FileID)
		}
		if sym.LocalSymbolID != localID {
			t.Errorf("Expected LocalSymbolID %d, got %d", localID, sym.LocalSymbolID)
		}
	})

	t.Run("String", func(t *testing.T) {
		sym := NewCompositeSymbolID(FileID(10), uint32(20))
		// String() returns human-readable debug format
		str := sym.String()
		expected := "Symbol[F:10,L:20]"

		if str != expected {
			t.Errorf("Expected string %q, got %q", expected, str)
		}

		// CompactString() should return dense encoding
		compact := sym.CompactString()
		if compact == "" {
			t.Error("Expected non-empty compact string")
		}

		// They should be different
		if str == compact {
			t.Error("String() and CompactString() should return different formats")
		}
	})

	t.Run("Equals", func(t *testing.T) {
		sym1 := NewCompositeSymbolID(FileID(1), uint32(2))
		sym2 := NewCompositeSymbolID(FileID(1), uint32(2))
		sym3 := NewCompositeSymbolID(FileID(1), uint32(3))
		sym4 := NewCompositeSymbolID(FileID(2), uint32(2))

		if !sym1.Equals(sym2) {
			t.Error("Expected sym1 to equal sym2")
		}
		if sym1.Equals(sym3) {
			t.Error("Expected sym1 to not equal sym3 (different LocalSymbolID)")
		}
		if sym1.Equals(sym4) {
			t.Error("Expected sym1 to not equal sym4 (different FileID)")
		}
	})

	t.Run("IsValid", func(t *testing.T) {
		validSym := NewCompositeSymbolID(FileID(1), uint32(0))
		invalidSym := NewCompositeSymbolID(FileID(0), uint32(0))
		validSym2 := NewCompositeSymbolID(FileID(0), uint32(1))

		if !validSym.IsValid() {
			t.Error("Expected symbol with FileID to be valid")
		}
		if invalidSym.IsValid() {
			t.Error("Expected zero symbol to be invalid")
		}
		if !validSym2.IsValid() {
			t.Error("Expected symbol with LocalSymbolID to be valid")
		}
	})

	t.Run("Hash", func(t *testing.T) {
		sym1 := NewCompositeSymbolID(FileID(1), uint32(2))
		sym2 := NewCompositeSymbolID(FileID(1), uint32(2))
		sym3 := NewCompositeSymbolID(FileID(2), uint32(1))

		hash1 := sym1.Hash()
		hash2 := sym2.Hash()
		hash3 := sym3.Hash()

		if hash1 != hash2 {
			t.Error("Expected equal symbols to have equal hashes")
		}
		if hash1 == hash3 {
			t.Error("Expected different symbols to have different hashes (not guaranteed but very likely)")
		}
	})

	t.Run("Hash Consistency", func(t *testing.T) {
		sym := NewCompositeSymbolID(FileID(42), uint32(123))

		hash1 := sym.Hash()
		hash2 := sym.Hash()

		if hash1 != hash2 {
			t.Error("Expected hash to be consistent for same symbol")
		}
	})
}

// TestSymbolScopeType tests the symbol scope type.
func TestSymbolScopeType(t *testing.T) {
	tests := []struct {
		scope    SymbolScopeType
		expected string
	}{
		{ScopeGlobal, "global"},
		{ScopeModule, "module"},
		{ScopePackage, "package"},
		{ScopeClass, "class"},
		{ScopeFunction, "function"},
		{ScopeMethod, "method"},
		{ScopeBlock, "block"},
		{ScopeNamespace, "namespace"},
		{SymbolScopeType(999), "unknown"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			if test.scope.String() != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, test.scope.String())
			}
		})
	}
}

// TestSymbolKind tests the symbol kind.
func TestSymbolKind(t *testing.T) {
	tests := []struct {
		kind     SymbolKind
		expected string
	}{
		{SymbolKindPackage, "package"},
		{SymbolKindImport, "import"},
		{SymbolKindType, "type"},
		{SymbolKindInterface, "interface"},
		{SymbolKindStruct, "struct"},
		{SymbolKindClass, "class"},
		{SymbolKindFunction, "function"},
		{SymbolKindMethod, "method"},
		{SymbolKindVariable, "variable"},
		{SymbolKindConstant, "constant"},
		{SymbolKindField, "field"},
		{SymbolKindProperty, "property"},
		{SymbolKindParameter, "parameter"},
		{SymbolKindLabel, "label"},
		{SymbolKindModule, "module"},
		{SymbolKindNamespace, "namespace"},
		{SymbolKindEnum, "enum"},
		{SymbolKindEnumMember, "enum_member"},
		{SymbolKindUnknown, "unknown"},
		{SymbolKind(999), "unknown"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			if test.kind.String() != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, test.kind.String())
			}
		})
	}
}

// TestResolutionType tests the resolution type.
func TestResolutionType(t *testing.T) {
	tests := []struct {
		resolution ResolutionType
		expected   string
	}{
		{ResolutionFile, "file"},
		{ResolutionDirectory, "directory"},
		{ResolutionPackage, "package"},
		{ResolutionModule, "module"},
		{ResolutionBuiltin, "builtin"},
		{ResolutionExternal, "external"},
		{ResolutionNotFound, "not_found"},
		{ResolutionUnknown, "unknown"},
		{ResolutionType(999), "unknown"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			if test.resolution.String() != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, test.resolution.String())
			}
		})
	}
}

// TestSymbolTable tests the symbol table.
func TestSymbolTable(t *testing.T) {
	t.Run("Initialization", func(t *testing.T) {
		table := &SymbolTable{
			FileID:        FileID(1),
			Language:      "go",
			Symbols:       make(map[uint32]*EnhancedSymbolInfo),
			SymbolsByName: make(map[string][]uint32),
			NextLocalID:   1,
		}

		if table.FileID != FileID(1) {
			t.Errorf("Expected FileID 1, got %d", table.FileID)
		}
		if table.Language != "go" {
			t.Errorf("Expected language 'go', got %q", table.Language)
		}
		if table.NextLocalID != 1 {
			t.Errorf("Expected NextLocalID 1, got %d", table.NextLocalID)
		}
	})

	t.Run("Symbol Storage", func(t *testing.T) {
		table := &SymbolTable{
			FileID:        FileID(1),
			Symbols:       make(map[uint32]*EnhancedSymbolInfo),
			SymbolsByName: make(map[string][]uint32),
			NextLocalID:   1,
		}

		// Add a symbol
		sym := &EnhancedSymbolInfo{
			ID:   NewCompositeSymbolID(FileID(1), uint32(1)),
			Name: "TestFunction",
			Kind: SymbolKindFunction,
		}

		table.Symbols[1] = sym
		table.SymbolsByName["TestFunction"] = []uint32{1}

		// Verify storage
		if stored, ok := table.Symbols[1]; !ok {
			t.Error("Symbol not found in table")
		} else if stored.Name != "TestFunction" {
			t.Errorf("Expected symbol name 'TestFunction', got %q", stored.Name)
		}

		if ids, ok := table.SymbolsByName["TestFunction"]; !ok {
			t.Error("Symbol not found by name")
		} else if len(ids) != 1 || ids[0] != 1 {
			t.Errorf("Expected symbol ID [1], got %v", ids)
		}
	})
}

// TestSymbolReference tests the symbol reference.
func TestSymbolReference(t *testing.T) {
	t.Run("External Reference", func(t *testing.T) {
		ref := SymbolReference{
			Symbol:     NewCompositeSymbolID(FileID(1), uint32(2)),
			Location:   SymbolLocation{FileID: FileID(3), Line: 10, Column: 5},
			IsExternal: true,
			ImportPath: "fmt",
		}

		if !ref.IsExternal {
			t.Error("Expected reference to be external")
		}
		if ref.ImportPath != "fmt" {
			t.Errorf("Expected import path 'fmt', got %q", ref.ImportPath)
		}
	})

	t.Run("Internal Reference", func(t *testing.T) {
		ref := SymbolReference{
			Symbol:     NewCompositeSymbolID(FileID(1), uint32(2)),
			Location:   SymbolLocation{FileID: FileID(1), Line: 20, Column: 10},
			IsExternal: false,
		}

		if ref.IsExternal {
			t.Error("Expected reference to be internal")
		}
		if ref.ImportPath != "" {
			t.Errorf("Expected empty import path, got %q", ref.ImportPath)
		}
	})
}

// TestImportInfo tests the import info.
func TestImportInfo(t *testing.T) {
	t.Run("Named Import", func(t *testing.T) {
		imp := ImportInfo{
			LocalID:       1,
			ImportPath:    "fmt",
			ImportedNames: []string{"Printf", "Println"},
			Location:      SymbolLocation{FileID: FileID(1), Line: 3},
		}

		if imp.ImportPath != "fmt" {
			t.Errorf("Expected import path 'fmt', got %q", imp.ImportPath)
		}
		if len(imp.ImportedNames) != 2 {
			t.Errorf("Expected 2 imported names, got %d", len(imp.ImportedNames))
		}
	})

	t.Run("Default Import", func(t *testing.T) {
		imp := ImportInfo{
			LocalID:    2,
			ImportPath: "react",
			Alias:      "React",
			IsDefault:  true,
			Location:   SymbolLocation{FileID: FileID(2), Line: 1},
		}

		if !imp.IsDefault {
			t.Error("Expected default import")
		}
		if imp.Alias != "React" {
			t.Errorf("Expected alias 'React', got %q", imp.Alias)
		}
	})

	t.Run("Namespace Import", func(t *testing.T) {
		imp := ImportInfo{
			LocalID:     3,
			ImportPath:  "./utils",
			Alias:       "utils",
			IsNamespace: true,
			Location:    SymbolLocation{FileID: FileID(3), Line: 2},
		}

		if !imp.IsNamespace {
			t.Error("Expected namespace import")
		}
		if imp.Alias != "utils" {
			t.Errorf("Expected alias 'utils', got %q", imp.Alias)
		}
	})
}

// TestExportInfo tests the export info.
func TestExportInfo(t *testing.T) {
	t.Run("Default Export", func(t *testing.T) {
		exp := ExportInfo{
			LocalID:      1,
			ExportedName: "default",
			LocalName:    "MyComponent",
			IsDefault:    true,
			Location:     SymbolLocation{FileID: FileID(1), Line: 50},
		}

		if !exp.IsDefault {
			t.Error("Expected default export")
		}
		if exp.LocalName != "MyComponent" {
			t.Errorf("Expected local name 'MyComponent', got %q", exp.LocalName)
		}
	})

	t.Run("Re-export", func(t *testing.T) {
		exp := ExportInfo{
			LocalID:      2,
			ExportedName: "Button",
			IsReExport:   true,
			SourcePath:   "./components/Button",
			Location:     SymbolLocation{FileID: FileID(2), Line: 10},
		}

		if !exp.IsReExport {
			t.Error("Expected re-export")
		}
		if exp.SourcePath != "./components/Button" {
			t.Errorf("Expected source path './components/Button', got %q", exp.SourcePath)
		}
	})
}

func BenchmarkCompositeSymbolIDHash(b *testing.B) {
	sym := NewCompositeSymbolID(FileID(42), uint32(123))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sym.Hash()
	}
}

func BenchmarkCompositeSymbolIDEquals(b *testing.B) {
	sym1 := NewCompositeSymbolID(FileID(42), uint32(123))
	sym2 := NewCompositeSymbolID(FileID(42), uint32(123))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sym1.Equals(sym2)
	}
}

// TestCompositeSymbolID_CompactEncoding tests the compact encoding functionality
func TestCompositeSymbolID_CompactEncoding(t *testing.T) {
	t.Run("Compact encoding round-trip", func(t *testing.T) {
		testCases := []struct {
			fileID        FileID
			localSymbolID uint32
			name          string
		}{
			{1, 1, "simple values"},
			{100, 200, "medium values"},
			{1000, 5000, "larger values"},
			{65535, 65535, "max uint16"},
			{1234567, 7654321, "large values"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create ID
				original := NewCompositeSymbolID(tc.fileID, tc.localSymbolID)

				// Encode to compact
				compact := original.CompactString()
				if compact == "" {
					t.Fatal("Expected non-empty compact string")
				}

				// Decode back using public API
				decoded, err := ParseCompactString(compact)
				if err != nil {
					t.Fatalf("Failed to decode: %v", err)
				}

				// Verify
				if decoded.FileID != tc.fileID {
					t.Errorf("FileID mismatch: want %d, got %d", tc.fileID, decoded.FileID)
				}
				if decoded.LocalSymbolID != tc.localSymbolID {
					t.Errorf("LocalSymbolID mismatch: want %d, got %d", tc.localSymbolID, decoded.LocalSymbolID)
				}
			})
		}
	})

	t.Run("JSON marshaling uses compact", func(t *testing.T) {
		original := NewCompositeSymbolID(FileID(123), uint32(456))

		// Marshal to JSON
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		// Should be a compact string in JSON
		if data[0] != '"' || data[len(data)-1] != '"' {
			t.Errorf("Expected JSON string, got: %s", string(data))
		}

		// Unmarshal back
		var decoded CompositeSymbolID
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify equals
		if !decoded.Equals(original) {
			t.Error("Decoded ID should equal original")
		}
	})

	t.Run("Backward compatibility with old JSON", func(t *testing.T) {
		// Old format JSON
		oldJSON := `{"file_id": 789, "local_symbol_id": 321}`

		var decoded CompositeSymbolID
		err := json.Unmarshal([]byte(oldJSON), &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal old format: %v", err)
		}

		if decoded.FileID != 789 {
			t.Errorf("FileID mismatch: want 789, got %d", decoded.FileID)
		}
		if decoded.LocalSymbolID != 321 {
			t.Errorf("LocalSymbolID mismatch: want 321, got %d", decoded.LocalSymbolID)
		}
	})

	t.Run("Character set validation", func(t *testing.T) {
		// Test various IDs to ensure valid characters
		for i := 0; i < 100; i++ {
			id := NewCompositeSymbolID(FileID(i*7), uint32(i*13))
			compact := id.CompactString()

			for _, c := range compact {
				valid := (c >= 'A' && c <= 'Z') ||
					(c >= 'a' && c <= 'z') ||
					(c >= '0' && c <= '9') ||
					c == '_'
				if !valid {
					t.Errorf("Invalid character '%c' in compact encoding", c)
				}
			}
		}
	})

	t.Run("Zero values", func(t *testing.T) {
		zero := NewCompositeSymbolID(0, 0)
		compact := zero.CompactString()

		if compact != "" {
			t.Errorf("Expected empty string for zero values, got %q", compact)
		}
	})

	t.Run("Compact length efficiency", func(t *testing.T) {
		// Compare with hex encoding
		id := NewCompositeSymbolID(FileID(1234567), uint32(7654321))
		compact := id.CompactString()

		// Hex would be 16 characters (8 bytes * 2)
		hexLen := 16
		if len(compact) >= hexLen {
			t.Errorf("Compact encoding (%d chars) should be shorter than hex (%d chars)", len(compact), hexLen)
		}

		t.Logf("Compact: %s (len=%d), would be %d in hex", compact, len(compact), hexLen)
	})
}

func TestCompositeSymbolID_ImportJSON(t *testing.T) {
	// Create a SymbolReference with CompositeSymbolID to test nested marshaling
	ref := SymbolReference{
		Symbol: NewCompositeSymbolID(FileID(42), uint32(99)),
		Location: SymbolLocation{
			FileID: FileID(10),
			Line:   25,
			Column: 10,
		},
		IsExternal: true,
		ImportPath: "fmt",
	}

	// Marshal the whole structure
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("Failed to marshal SymbolReference: %v", err)
	}

	// Unmarshal back
	var decoded SymbolReference
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal SymbolReference: %v", err)
	}

	// Verify the CompositeSymbolID was preserved
	if !decoded.Symbol.Equals(ref.Symbol) {
		t.Error("Symbol ID not preserved through JSON round-trip")
	}
}

func BenchmarkCompositeSymbolID_CompactString(b *testing.B) {
	sym := NewCompositeSymbolID(FileID(12345), uint32(67890))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sym.CompactString()
	}
}

func BenchmarkCompositeSymbolID_JSONRoundTrip(b *testing.B) {
	sym := NewCompositeSymbolID(FileID(12345), uint32(67890))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(sym)
		var decoded CompositeSymbolID
		_ = json.Unmarshal(data, &decoded)
	}
}
