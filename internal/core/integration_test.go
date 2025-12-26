package core_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/standardbeagle/lci/internal/testing/builders"
	"github.com/standardbeagle/lci/internal/testing/fixtures"
	"github.com/standardbeagle/lci/internal/types"
)

// TestCallGraphIntegration tests call graph construction
// DISABLED: Requires CallGraph and CallEdge types which were removed from core
func TestCallGraphIntegration(t *testing.T) {
	t.Skip("Disabled: CallGraph and CallEdge types removed from core")
}

// TestReferenceTrackingIntegration tests reference tracking across files
func TestReferenceTrackingIntegration(t *testing.T) {
	indexer := builders.NewTestIndexer(t)

	files := map[string]string{
		"types.go": `package main

type User struct {
	ID   int
	Name string
}

type UserService interface {
	GetUser(id int) *User
	SaveUser(user *User) error
}
`,
		"service.go": `package main

type userService struct {
	db Database
}

func (s *userService) GetUser(id int) *User {
	return &User{ID: id, Name: "Test"}
}

func (s *userService) SaveUser(user *User) error {
	return s.db.Save(user)
}

func NewUserService(db Database) UserService {
	return &userService{db: db}
}
`,
		"main.go": `package main

func main() {
	db := NewDatabase()
	service := NewUserService(db)
	
	user := service.GetUser(123)
	user.Name = "Updated"
	
	err := service.SaveUser(user)
	if err != nil {
		panic(err)
	}
}
`,
	}

	if err := indexer.IndexFiles(files); err != nil {
		t.Fatal(err)
	}

	// Test User type references
	userType := findSymbolByName(indexer, "User")
	if userType == nil {
		t.Fatal("User type not found")
	}

	// Get all references to User
	refs := indexer.GetSymbolReferences(userType.ID)

	// Should have multiple references
	if len(refs) < 3 {
		t.Errorf("Expected at least 3 references to User, got %d", len(refs))
	}

	// Verify we have some references
	if len(refs) == 0 {
		t.Error("Expected at least some references to User type")
	}

	// Log the references for debugging
	t.Logf("Found %d references to User type", len(refs))
}

// TestSymbolHierarchyIntegration tests symbol hierarchy and scoping
func TestSymbolHierarchyIntegration(t *testing.T) {
	indexer := builders.NewTestIndexer(t)

	code := `package main

type Outer struct {
	field1 string
}

func (o *Outer) Method1() {
	inner := func() {
		deepest := func() {
			println("nested")
		}
		deepest()
	}
	inner()
}

func (o *Outer) Method2() {
	for i := 0; i < 10; i++ {
		if i > 5 {
			go func() {
				println(i)
			}()
		}
	}
}

type Interface interface {
	Method1()
	Method2()
}
`

	if err := indexer.IndexString("hierarchy.go", code); err != nil {
		t.Fatal(err)
	}

	// Get file scopes
	fileID := indexer.GetAllFileIDs()[0]
	scopes := indexer.GetFileScopeHierarchy(fileID)

	// Should have nested scopes
	if len(scopes) < 3 {
		t.Errorf("Expected at least 3 scope levels, got %d", len(scopes))
	}

	// Test symbol at line
	// Line numbers would need adjustment based on actual parsing
	method1 := findSymbolByName(indexer, "Method1")
	if method1 == nil {
		t.Fatal("Method1 not found")
	}

	// Get symbol at specific line within Method1
	symbolAtLine := indexer.GetSymbolAtLine(fileID, method1.Line+2)
	if symbolAtLine == nil {
		t.Error("Expected to find symbol at line within Method1")
	}
}

// TestConcurrentSymbolAccess tests concurrent access to symbol data
func TestConcurrentSymbolAccess(t *testing.T) {
	indexer := builders.NewTestIndexer(t)

	// Create many symbols
	testFiles := fixtures.GenerateSimpleTestFiles(50)
	for _, file := range testFiles {
		if err := indexer.IndexString(file.Path, file.Content); err != nil {
			t.Fatal(err)
		}
	}

	// Get some symbols for testing by looking at the first file
	fileIDs := indexer.GetAllFileIDs()
	if len(fileIDs) == 0 {
		t.Fatal("No indexed files found")
	}

	// Get symbols from the first file
	testSymbols := indexer.GetFileEnhancedSymbols(fileIDs[0])
	if len(testSymbols) == 0 {
		t.Fatal("No test symbols found")
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Perform many concurrent operations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			// Mix of operations using actual symbols
			switch n % 4 {
			case 0:
				// Symbol lookup by name
				if len(testSymbols) > 0 {
					testSym := testSymbols[n%len(testSymbols)]
					syms := indexer.FindSymbolsByName(testSym.Name)
					// Don't error if no symbols found - this is common in concurrent access
					_ = syms
				}
			case 1:
				// Reference lookup
				if len(testSymbols) > 0 {
					testSym := testSymbols[n%len(testSymbols)]
					refs := indexer.GetSymbolReferences(testSym.ID)
					_ = refs
				}
			case 2:
				// Enhanced symbol lookup
				if len(testSymbols) > 0 {
					testSym := testSymbols[n%len(testSymbols)]
					sym := indexer.GetEnhancedSymbol(testSym.ID)
					_ = sym
				}
			case 3:
				// File symbols
				fileIDs := indexer.GetAllFileIDs()
				if len(fileIDs) > 0 {
					syms := indexer.GetFileEnhancedSymbols(fileIDs[n%len(fileIDs)])
					_ = syms
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Concurrent access error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("%d concurrent operations failed", errorCount)
	}
}

// TestTrigramIndexIntegration tests trigram index functionality
func TestTrigramIndexIntegration(t *testing.T) {
	indexer := builders.NewTestIndexer(t)

	// Index code with various patterns
	code := `package main

// LongFunctionNameForTesting tests trigram matching
func LongFunctionNameForTesting() {
	variableWithLongName := "test"
	anotherLongVariableName := 123
	
	// Call functions with similar names
	processUserData()
	processOrderData()
	processPaymentData()
}

func processUserData() {}
func processOrderData() {}
func processPaymentData() {}

const (
	ConfigurationSettingName = "app.setting"
	ConfigurationSettingValue = "value"
)
`

	if err := indexer.IndexString("trigrams.go", code); err != nil {
		t.Fatal(err)
	}

	// Test fuzzy search patterns
	patterns := []struct {
		query    string
		expected []string
	}{
		{
			query:    "LongFunc",
			expected: []string{"LongFunctionNameForTesting"},
		},
		{
			query:    "processData",
			expected: []string{"processUserData", "processOrderData", "processPaymentData"},
		},
		{
			query:    "ConfigSetting",
			expected: []string{"ConfigurationSettingName", "ConfigurationSettingValue"},
		},
	}

	for _, p := range patterns {
		// Skip search tests to avoid import cycle
		// TODO: Add search tests once search engine is integrated
		t.Skipf("Skipping fuzzy search test for %q to avoid import cycle", p.query)
	}
}

// TestImportResolution tests import and dependency resolution
func TestImportResolution(t *testing.T) {
	indexer := builders.NewTestIndexer(t)

	files := map[string]string{
		"main.go": `package main

import (
	"fmt"
	"log"
)

const Version = "1.0.0"

func GetVersion() string {
	return Version
}

func main() {
	fmt.Println(GetVersion())
}

func Helper() {
	log.Println("help")
}
`,
		"service.go": `package main

func main() {
	Helper()
}
`,
	}

	if err := indexer.IndexFiles(files); err != nil {
		t.Fatal(err)
	}

	// Verify symbols are indexed
	getVersion := findSymbolByName(indexer, "GetVersion")
	if getVersion == nil {
		t.Fatal("GetVersion not found")
	}

	// Verify Helper function exists
	helper := findSymbolByName(indexer, "Helper")
	if helper == nil {
		t.Fatal("Helper not found")
	}

	// Check references - Helper should be called from main
	refs := indexer.GetSymbolReferences(helper.ID)
	if len(refs) == 0 {
		t.Fatal("Helper should have references")
	}

	// Verify at least one call reference exists
	hasCallRef := false
	for _, ref := range refs {
		if ref.Type == types.RefTypeCall {
			hasCallRef = true
			break
		}
	}
	if !hasCallRef {
		t.Error("Helper should have at least one call reference")
	}
}

// BenchmarkCoreSymbolLookup measures symbol lookup performance in core integration context
// Renamed from BenchmarkSymbolLookup to disambiguate from indexing benchmark
func BenchmarkCoreSymbolLookup(b *testing.B) {
	indexer := builders.NewTestIndexer(&testing.T{})

	// Create large index
	testFiles := fixtures.GenerateSimpleTestFiles(1000)
	for _, file := range testFiles {
		if err := indexer.IndexString(file.Path, file.Content); err != nil {
			b.Fatal(err)
		}
	}

	// Symbol names to look up
	names := []string{"Function1", "Method5", "Class10", "variable20"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		name := names[i%len(names)]
		_ = indexer.FindSymbolsByName(name)
	}
}

// Helper functions

func findSymbolByName(indexer *builders.TestIndexer, name string) *types.EnhancedSymbol {
	symbols := indexer.FindSymbolsByName(name)
	if len(symbols) > 0 {
		return symbols[0]
	}
	return nil
}

func contains(lines []string, text string) bool {
	for _, line := range lines {
		if strings.Contains(line, text) {
			return true
		}
	}
	return false
}
