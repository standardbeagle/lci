package core

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestReferenceTracker_TypeRelationships tests the type hierarchy query methods
func TestReferenceTracker_TypeRelationships(t *testing.T) {
	rt := NewReferenceTrackerForTest()

	// Create test symbols
	symbols := []types.Symbol{
		{Name: "InterfaceA", Type: types.SymbolTypeInterface, Line: 10, EndLine: 15},
		{Name: "ClassB", Type: types.SymbolTypeClass, Line: 20, EndLine: 25},
		{Name: "ClassC", Type: types.SymbolTypeClass, Line: 30, EndLine: 35},
		{Name: "BaseClass", Type: types.SymbolTypeClass, Line: 40, EndLine: 45},
		{Name: "DerivedClass", Type: types.SymbolTypeClass, Line: 50, EndLine: 55},
	}

	// Process file to register symbols
	enhancedSymbols := rt.ProcessFile(1, "test.go", symbols, nil, nil)

	// Get symbol IDs from enhanced symbols
	var interfaceAID, classBID, classCID, baseClassID, derivedClassID types.SymbolID
	for _, sym := range enhancedSymbols {
		switch sym.Name {
		case "InterfaceA":
			interfaceAID = sym.ID
		case "ClassB":
			classBID = sym.ID
		case "ClassC":
			classCID = sym.ID
		case "BaseClass":
			baseClassID = sym.ID
		case "DerivedClass":
			derivedClassID = sym.ID
		}
	}

	// Add implements references: ClassB and ClassC implement InterfaceA
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   classBID,
		TargetSymbol:   interfaceAID,
		ReferencedName: "InterfaceA",
		FileID:         1,
		Line:           21,
	})
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   classCID,
		TargetSymbol:   interfaceAID,
		ReferencedName: "InterfaceA",
		FileID:         1,
		Line:           31,
	})

	// Add extends reference: DerivedClass extends BaseClass
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeExtends,
		SourceSymbol:   derivedClassID,
		TargetSymbol:   baseClassID,
		ReferencedName: "BaseClass",
		FileID:         1,
		Line:           51,
	})

	t.Run("GetImplementors", func(t *testing.T) {
		implementors := rt.GetImplementors(interfaceAID)
		if len(implementors) != 2 {
			t.Errorf("expected 2 implementors, got %d", len(implementors))
		}

		// Verify the implementors are ClassB and ClassC
		found := make(map[types.SymbolID]bool)
		for _, id := range implementors {
			found[id] = true
		}
		if !found[classBID] {
			t.Error("expected ClassB to be an implementor")
		}
		if !found[classCID] {
			t.Error("expected ClassC to be an implementor")
		}
	})

	t.Run("GetImplementedInterfaces", func(t *testing.T) {
		interfaces := rt.GetImplementedInterfaces(classBID)
		if len(interfaces) != 1 {
			t.Errorf("expected 1 interface, got %d", len(interfaces))
		}
		if interfaces[0] != interfaceAID {
			t.Errorf("expected InterfaceA, got %v", interfaces[0])
		}
	})

	t.Run("GetBaseTypes", func(t *testing.T) {
		bases := rt.GetBaseTypes(derivedClassID)
		if len(bases) != 1 {
			t.Errorf("expected 1 base type, got %d", len(bases))
		}
		if bases[0] != baseClassID {
			t.Errorf("expected BaseClass, got %v", bases[0])
		}
	})

	t.Run("GetDerivedTypes", func(t *testing.T) {
		derived := rt.GetDerivedTypes(baseClassID)
		if len(derived) != 1 {
			t.Errorf("expected 1 derived type, got %d", len(derived))
		}
		if derived[0] != derivedClassID {
			t.Errorf("expected DerivedClass, got %v", derived[0])
		}
	})

	t.Run("GetTypeRelationships", func(t *testing.T) {
		// Test for interface - should have implementors
		ifaceRels := rt.GetTypeRelationships(interfaceAID)
		if ifaceRels == nil {
			t.Fatal("expected non-nil type relationships")
		}
		if len(ifaceRels.ImplementedBy) != 2 {
			t.Errorf("expected 2 ImplementedBy, got %d", len(ifaceRels.ImplementedBy))
		}

		// Test for ClassB - should implement InterfaceA
		classBRels := rt.GetTypeRelationships(classBID)
		if len(classBRels.Implements) != 1 {
			t.Errorf("expected 1 Implements, got %d", len(classBRels.Implements))
		}

		// Test for base class - should have derived types
		baseRels := rt.GetTypeRelationships(baseClassID)
		if len(baseRels.ExtendedBy) != 1 {
			t.Errorf("expected 1 ExtendedBy, got %d", len(baseRels.ExtendedBy))
		}

		// Test for derived class - should have base types
		derivedRels := rt.GetTypeRelationships(derivedClassID)
		if len(derivedRels.Extends) != 1 {
			t.Errorf("expected 1 Extends, got %d", len(derivedRels.Extends))
		}
	})

	t.Run("TypeRelationships_HasTypeRelationships", func(t *testing.T) {
		ifaceRels := rt.GetTypeRelationships(interfaceAID)
		if !ifaceRels.HasTypeRelationships() {
			t.Error("expected interface to have type relationships")
		}

		// Symbol with no relationships - add new orphan symbol
		orphanSymbols := []types.Symbol{
			{Name: "Orphan", Type: types.SymbolTypeClass, Line: 100, EndLine: 110},
		}
		orphanEnhanced := rt.ProcessFile(2, "orphan.go", orphanSymbols, nil, nil)
		orphanID := orphanEnhanced[0].ID

		orphanRels := rt.GetTypeRelationships(orphanID)
		if orphanRels.HasTypeRelationships() {
			t.Error("expected orphan to have no type relationships")
		}
	})

	t.Run("FindSymbolByName", func(t *testing.T) {
		found := rt.FindSymbolByName("InterfaceA")
		if found == nil {
			t.Fatal("expected to find InterfaceA")
		}
		if found.ID != interfaceAID {
			t.Errorf("expected ID %d, got %d", interfaceAID, found.ID)
		}

		notFound := rt.FindSymbolByName("NonExistent")
		if notFound != nil {
			t.Error("expected nil for non-existent symbol")
		}
	})

	t.Run("FindSymbolByFileAndName", func(t *testing.T) {
		found := rt.FindSymbolByFileAndName(1, "ClassB")
		if found == nil {
			t.Fatal("expected to find ClassB")
		}
		if found.ID != classBID {
			t.Errorf("expected ID %d, got %d", classBID, found.ID)
		}

		// Wrong file
		notFound := rt.FindSymbolByFileAndName(999, "ClassB")
		if notFound != nil {
			t.Error("expected nil for wrong file ID")
		}
	})
}

// TestReferenceTracker_NoTypeRelationships ensures no false positives
func TestReferenceTracker_NoTypeRelationships(t *testing.T) {
	rt := NewReferenceTrackerForTest()

	// Create symbols with no type relationships
	symbols := []types.Symbol{
		{Name: "FuncA", Type: types.SymbolTypeFunction, Line: 10, EndLine: 20},
		{Name: "FuncB", Type: types.SymbolTypeFunction, Line: 30, EndLine: 40},
	}

	enhancedSymbols := rt.ProcessFile(1, "test.go", symbols, nil, nil)
	funcAID := enhancedSymbols[0].ID
	funcBID := enhancedSymbols[1].ID

	// Add a call reference (not a type relationship)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeCall,
		SourceSymbol:   funcAID,
		TargetSymbol:   funcBID,
		ReferencedName: "FuncB",
		FileID:         1,
		Line:           15,
	})

	// Type relationship queries should return empty
	implementors := rt.GetImplementors(funcAID)
	if len(implementors) != 0 {
		t.Errorf("expected 0 implementors, got %d", len(implementors))
	}

	interfaces := rt.GetImplementedInterfaces(funcAID)
	if len(interfaces) != 0 {
		t.Errorf("expected 0 interfaces, got %d", len(interfaces))
	}

	bases := rt.GetBaseTypes(funcAID)
	if len(bases) != 0 {
		t.Errorf("expected 0 base types, got %d", len(bases))
	}

	derived := rt.GetDerivedTypes(funcAID)
	if len(derived) != 0 {
		t.Errorf("expected 0 derived types, got %d", len(derived))
	}
}

// TestReferenceTracker_MultipleInheritance tests complex inheritance scenarios
func TestReferenceTracker_MultipleInheritance(t *testing.T) {
	rt := NewReferenceTrackerForTest()

	// Create symbols for multiple inheritance scenario
	symbols := []types.Symbol{
		{Name: "Interface1", Type: types.SymbolTypeInterface, Line: 10, EndLine: 15},
		{Name: "Interface2", Type: types.SymbolTypeInterface, Line: 20, EndLine: 25},
		{Name: "BaseA", Type: types.SymbolTypeClass, Line: 30, EndLine: 35},
		{Name: "BaseB", Type: types.SymbolTypeClass, Line: 40, EndLine: 45},
		{Name: "Derived", Type: types.SymbolTypeClass, Line: 50, EndLine: 55},
	}

	enhancedSymbols := rt.ProcessFile(1, "test.go", symbols, nil, nil)

	// Get symbol IDs
	var iface1ID, iface2ID, baseAID, baseBID, derivedID types.SymbolID
	for _, sym := range enhancedSymbols {
		switch sym.Name {
		case "Interface1":
			iface1ID = sym.ID
		case "Interface2":
			iface2ID = sym.ID
		case "BaseA":
			baseAID = sym.ID
		case "BaseB":
			baseBID = sym.ID
		case "Derived":
			derivedID = sym.ID
		}
	}

	// Derived implements both interfaces
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   derivedID,
		TargetSymbol:   iface1ID,
		ReferencedName: "Interface1",
		FileID:         1,
		Line:           51,
	})
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   derivedID,
		TargetSymbol:   iface2ID,
		ReferencedName: "Interface2",
		FileID:         1,
		Line:           52,
	})

	// Derived extends BaseA (Go-style embedding)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeExtends,
		SourceSymbol:   derivedID,
		TargetSymbol:   baseAID,
		ReferencedName: "BaseA",
		FileID:         1,
		Line:           53,
	})

	// Derived also extends BaseB (multiple embedding)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeExtends,
		SourceSymbol:   derivedID,
		TargetSymbol:   baseBID,
		ReferencedName: "BaseB",
		FileID:         1,
		Line:           54,
	})

	t.Run("MultipleInterfaces", func(t *testing.T) {
		interfaces := rt.GetImplementedInterfaces(derivedID)
		if len(interfaces) != 2 {
			t.Errorf("expected 2 interfaces, got %d", len(interfaces))
		}

		found := make(map[types.SymbolID]bool)
		for _, id := range interfaces {
			found[id] = true
		}
		if !found[iface1ID] || !found[iface2ID] {
			t.Error("expected both interfaces to be found")
		}
	})

	t.Run("MultipleBases", func(t *testing.T) {
		bases := rt.GetBaseTypes(derivedID)
		if len(bases) != 2 {
			t.Errorf("expected 2 base types, got %d", len(bases))
		}

		found := make(map[types.SymbolID]bool)
		for _, id := range bases {
			found[id] = true
		}
		if !found[baseAID] || !found[baseBID] {
			t.Error("expected both base types to be found")
		}
	})

	t.Run("AllTypeRelationships", func(t *testing.T) {
		rels := rt.GetTypeRelationships(derivedID)
		if len(rels.Implements) != 2 {
			t.Errorf("expected 2 Implements, got %d", len(rels.Implements))
		}
		if len(rels.Extends) != 2 {
			t.Errorf("expected 2 Extends, got %d", len(rels.Extends))
		}
		if !rels.HasTypeRelationships() {
			t.Error("expected Derived to have type relationships")
		}
	})
}

// TestReferenceTracker_QualityRankedImplementors tests quality-aware implementation queries
func TestReferenceTracker_QualityRankedImplementors(t *testing.T) {
	rt := NewReferenceTrackerForTest()

	// Create test symbols: one interface and three implementors
	symbols := []types.Symbol{
		{Name: "Writer", Type: types.SymbolTypeInterface, Line: 10, EndLine: 15},
		{Name: "FileWriter", Type: types.SymbolTypeStruct, Line: 20, EndLine: 25},
		{Name: "BufferWriter", Type: types.SymbolTypeStruct, Line: 30, EndLine: 35},
		{Name: "NetWriter", Type: types.SymbolTypeStruct, Line: 40, EndLine: 45},
	}

	// Process file to register symbols
	enhancedSymbols := rt.ProcessFile(1, "test.go", symbols, nil, nil)

	// Get symbol IDs
	var writerID, fileWriterID, bufferWriterID, netWriterID types.SymbolID
	for _, sym := range enhancedSymbols {
		switch sym.Name {
		case "Writer":
			writerID = sym.ID
		case "FileWriter":
			fileWriterID = sym.ID
		case "BufferWriter":
			bufferWriterID = sym.ID
		case "NetWriter":
			netWriterID = sym.ID
		}
	}

	// Add implements references with different quality levels
	// FileWriter: assigned (highest)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   fileWriterID,
		TargetSymbol:   writerID,
		ReferencedName: "Writer",
		FileID:         1,
		Line:           21,
		Quality:        types.RefQualityAssigned,
	})

	// BufferWriter: heuristic (lowest)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   bufferWriterID,
		TargetSymbol:   writerID,
		ReferencedName: "Writer",
		FileID:         1,
		Line:           31,
		Quality:        types.RefQualityHeuristic,
	})

	// NetWriter: returned (medium)
	rt.AddTestReference(types.Reference{
		Type:           types.RefTypeImplements,
		SourceSymbol:   netWriterID,
		TargetSymbol:   writerID,
		ReferencedName: "Writer",
		FileID:         1,
		Line:           41,
		Quality:        types.RefQualityReturned,
	})

	// Process all references to build bidirectional maps
	rt.ProcessAllReferences()

	t.Run("GetImplementorsWithQuality_SortedByRank", func(t *testing.T) {
		implementors := rt.GetImplementorsWithQuality(writerID)

		if len(implementors) != 3 {
			t.Fatalf("expected 3 implementors, got %d", len(implementors))
		}

		// Should be sorted: assigned (95) > returned (90) > heuristic (50)
		if implementors[0].Quality != types.RefQualityAssigned {
			t.Errorf("expected first implementor to be 'assigned', got %q", implementors[0].Quality)
		}
		if implementors[1].Quality != types.RefQualityReturned {
			t.Errorf("expected second implementor to be 'returned', got %q", implementors[1].Quality)
		}
		if implementors[2].Quality != types.RefQualityHeuristic {
			t.Errorf("expected third implementor to be 'heuristic', got %q", implementors[2].Quality)
		}

		// Verify rank order
		for i := 0; i < len(implementors)-1; i++ {
			if implementors[i].Rank < implementors[i+1].Rank {
				t.Errorf("implementors not sorted by rank: %d < %d", implementors[i].Rank, implementors[i+1].Rank)
			}
		}
	})

	t.Run("GetImplementedInterfacesWithQuality", func(t *testing.T) {
		// FileWriter implements Writer with "assigned" quality
		interfaces := rt.GetImplementedInterfacesWithQuality(fileWriterID)

		if len(interfaces) != 1 {
			t.Fatalf("expected 1 interface, got %d", len(interfaces))
		}

		if interfaces[0].Quality != types.RefQualityAssigned {
			t.Errorf("expected quality 'assigned', got %q", interfaces[0].Quality)
		}
		if interfaces[0].SymbolID != writerID {
			t.Errorf("expected interface to be Writer")
		}
	})

	t.Run("HighestQualityWins", func(t *testing.T) {
		// Add a second reference for FileWriter with lower quality
		rt.AddTestReference(types.Reference{
			Type:           types.RefTypeImplements,
			SourceSymbol:   fileWriterID,
			TargetSymbol:   writerID,
			ReferencedName: "Writer",
			FileID:         1,
			Line:           22,
			Quality:        types.RefQualityHeuristic, // Lower quality than assigned
		})

		implementors := rt.GetImplementorsWithQuality(writerID)

		// FileWriter should still show "assigned" (highest quality wins)
		var fileWriterImpl *ImplementorWithQuality
		for _, impl := range implementors {
			if impl.SymbolID == fileWriterID {
				fileWriterImpl = &impl
				break
			}
		}

		if fileWriterImpl == nil {
			t.Fatal("FileWriter not found in implementors")
		}

		if fileWriterImpl.Quality != types.RefQualityAssigned {
			t.Errorf("expected highest quality 'assigned' for FileWriter, got %q", fileWriterImpl.Quality)
		}
	})
}
