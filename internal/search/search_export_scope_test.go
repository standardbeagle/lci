package search_test

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestExportScope_ExportedOnly tests ExportedOnly filter
func TestExportScope_ExportedOnly(t *testing.T) {
	code := `package main

// ExportedFunction is a public function
func ExportedFunction() {
	unexportedHelper()
}

// unexportedHelper is a private function
func unexportedHelper() {
	// Implementation
}

// ExportedStruct is a public struct
type ExportedStruct struct {
	PublicField  string
	privateField int
}

// unexportedStruct is a private struct
type unexportedStruct struct {
	field string
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Search for exported symbols only
	exportedOnly := engine.SearchWithOptions("Function", allFiles, types.SearchOptions{
		ExportedOnly: true,
	})

	// Search for all symbols
	allSymbols := engine.SearchWithOptions("Function", allFiles, types.SearchOptions{})

	t.Logf("ExportedOnly: %d results", len(exportedOnly))
	t.Logf("All symbols: %d results", len(allSymbols))

	// Exported-only should have fewer or equal results
	if len(exportedOnly) > len(allSymbols) {
		t.Errorf("ExportedOnly should not have more results than all symbols: got %d vs %d",
			len(exportedOnly), len(allSymbols))
	}
}

// TestExportScope_ExportedTypes tests ExportedOnly with type filtering
func TestExportScope_ExportedTypes(t *testing.T) {
	code := `package main

// PublicAPI is the public API
type PublicAPI struct {
	PublicField  string
	privateField int
}

// privateAPI is not exported
type privateAPI struct {
	field string
}

// PublicInterface is exported
type PublicInterface interface {
	Method()
}

// privateInterface is not exported
type privateInterface interface {
	method()
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Exported types only
	exportedTypes := engine.SearchWithOptions("API", allFiles, types.SearchOptions{
		ExportedOnly: true,
		SymbolTypes:  []string{"class"},
	})

	// All types
	allTypes := engine.SearchWithOptions("API", allFiles, types.SearchOptions{
		SymbolTypes: []string{"class"},
	})

	t.Logf("Exported types: %d, All types: %d", len(exportedTypes), len(allTypes))
}

// TestExportScope_ExportedFunctions tests ExportedOnly with function filtering
func TestExportScope_ExportedFunctions(t *testing.T) {
	code := `package main

// ProcessData is exported
func ProcessData(input string) error {
	return validate(input)
}

// validate is not exported
func validate(input string) error {
	return nil
}

// FormatOutput is exported
func FormatOutput(data string) string {
	return format(data)
}

func format(data string) string {
	return data
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Exported functions only
	exportedFuncs := engine.SearchWithOptions("Data", allFiles, types.SearchOptions{
		ExportedOnly: true,
		SymbolTypes:  []string{"function"},
	})

	// All functions
	allFuncs := engine.SearchWithOptions("Data", allFiles, types.SearchOptions{
		SymbolTypes: []string{"function"},
	})

	t.Logf("Exported functions: %d, All functions: %d", len(exportedFuncs), len(allFuncs))
}

// TestExportScope_ExportedVariables tests ExportedOnly with variable filtering
func TestExportScope_ExportedVariables(t *testing.T) {
	code := `package main

// PublicConfig is an exported variable
var PublicConfig = "default"

// privateConfig is not exported
var privateConfig = "internal"

// DebugMode is an exported constant
const DebugMode = true

const debugLevel = 1
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Exported variables only
	exportedVars := engine.SearchWithOptions("Config", allFiles, types.SearchOptions{
		ExportedOnly: true,
	})

	// All variables
	allVars := engine.SearchWithOptions("Config", allFiles, types.SearchOptions{})

	t.Logf("Exported variables: %d, All variables: %d", len(exportedVars), len(allVars))
}

// TestExportScope_WithDeclarationOnly tests combining ExportedOnly with DeclarationOnly
func TestExportScope_WithDeclarationOnly(t *testing.T) {
	code := `package main

// PublicHelper is exported
func PublicHelper() {
	privateHelper()
}

func privateHelper() {
	PublicHelper() // Usage of exported function
}

func main() {
	PublicHelper()
	privateHelper()
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Exported declarations only
	exportedDecl := engine.SearchWithOptions("Helper", allFiles, types.SearchOptions{
		ExportedOnly:    true,
		DeclarationOnly: true,
	})

	// Exported symbols (including usages)
	exportedAll := engine.SearchWithOptions("Helper", allFiles, types.SearchOptions{
		ExportedOnly: true,
	})

	// All declarations
	allDecl := engine.SearchWithOptions("Helper", allFiles, types.SearchOptions{
		DeclarationOnly: true,
	})

	t.Logf("Exported declarations: %d, Exported all: %d, All declarations: %d",
		len(exportedDecl), len(exportedAll), len(allDecl))
}

// TestExportScope_CaseInsensitive tests ExportedOnly with case insensitivity
func TestExportScope_CaseInsensitive(t *testing.T) {
	code := `package main

func PublicFunc() {}

func publicfunc() {}

func PrivateFunc() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Case-insensitive exported search
	exportedCI := engine.SearchWithOptions("publicfunc", allFiles, types.SearchOptions{
		ExportedOnly:    true,
		CaseInsensitive: true,
	})

	// Case-sensitive exported search
	exportedCS := engine.SearchWithOptions("PublicFunc", allFiles, types.SearchOptions{
		ExportedOnly:    true,
		CaseInsensitive: false,
	})

	t.Logf("Case-insensitive exported: %d, Case-sensitive exported: %d",
		len(exportedCI), len(exportedCS))
}

// TestExportScope_StructFields tests exported vs unexported struct fields
func TestExportScope_StructFields(t *testing.T) {
	code := `package main

type DataContainer struct {
	// PublicData is exported
	PublicData string
	// privateData is not exported
	privateData int
	// AnotherPublic is exported
	AnotherPublic bool
}

func useData() {
	dc := DataContainer{
		PublicData:    "test",
		privateData:   42,
		AnotherPublic: true,
	}
	_ = dc
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Exported fields only
	exportedFields := engine.SearchWithOptions("Data", allFiles, types.SearchOptions{
		ExportedOnly: true,
		SymbolTypes:  []string{"field"},
	})

	// All fields
	allFields := engine.SearchWithOptions("Data", allFiles, types.SearchOptions{
		SymbolTypes: []string{"field"},
	})

	t.Logf("Exported fields: %d, All fields: %d", len(exportedFields), len(allFields))
}

// TestExportScope_Methods tests exported vs unexported methods
func TestExportScope_Methods(t *testing.T) {
	code := `package main

type Processor struct {
	data string
}

// PublicMethod is exported
func (p *Processor) PublicMethod() error {
	return p.privateMethod()
}

// privateMethod is not exported
func (p *Processor) privateMethod() error {
	return nil
}

// AnotherPublicMethod is exported
func (p *Processor) AnotherPublicMethod() {
	p.privateMethod()
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Exported methods only
	exportedMethods := engine.SearchWithOptions("Method", allFiles, types.SearchOptions{
		ExportedOnly: true,
		SymbolTypes:  []string{"method"},
	})

	// All methods
	allMethods := engine.SearchWithOptions("Method", allFiles, types.SearchOptions{
		SymbolTypes: []string{"method"},
	})

	t.Logf("Exported methods: %d, All methods: %d", len(exportedMethods), len(allMethods))
}
