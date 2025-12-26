package search_test

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestRegexPatterns_BasicRegex tests basic regex pattern matching
func TestRegexPatterns_BasicRegex(t *testing.T) {
	code := `package main

func Test123() {}
func Test456() {}
func TestABC() {}
func Helper() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Match Test followed by digits
	digits := engine.SearchWithOptions("Test[0-9]+", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Match Test followed by letters
	letters := engine.SearchWithOptions("Test[A-Z]+", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Match any Test pattern
	anyTest := engine.SearchWithOptions("Test.*", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Test[0-9]+: %d, Test[A-Z]+: %d, Test.*: %d",
		len(digits), len(letters), len(anyTest))
}

// TestRegexPatterns_Anchors tests regex anchors (^, $)
func TestRegexPatterns_Anchors(t *testing.T) {
	code := `package main

func TestStart() {}
var EndTest = "test"
var TestMiddle = "test"
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Start anchor - may not work as expected in code search
	startAnchor := engine.SearchWithOptions("^Test", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// End anchor
	endAnchor := engine.SearchWithOptions("Test$", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("^Test: %d, Test$: %d", len(startAnchor), len(endAnchor))
}

// TestRegexPatterns_Quantifiers tests regex quantifiers (*, +, ?, {n})
func TestRegexPatterns_Quantifiers(t *testing.T) {
	code := `package main

func Test() {}
func Testt() {}
func Testtt() {}
func Testttt() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Zero or more 't'
	zeroOrMore := engine.SearchWithOptions("Test*", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// One or more 't'
	oneOrMore := engine.SearchWithOptions("Testt+", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Exactly 3 't's
	exactThree := engine.SearchWithOptions("Testt{3}", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Test*: %d, Testt+: %d, Testt{3}: %d",
		len(zeroOrMore), len(oneOrMore), len(exactThree))
}

// TestRegexPatterns_CharacterClasses tests character classes
func TestRegexPatterns_CharacterClasses(t *testing.T) {
	code := `package main

func TestFunc() {}
func test_func() {}
func TEST_FUNC() {}
func Test123Func() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Word characters
	wordChars := engine.SearchWithOptions(`Test\w+`, allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Digits
	digits := engine.SearchWithOptions(`Test\d+`, allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Whitespace (in identifiers, unlikely)
	whitespace := engine.SearchWithOptions(`Test\s+`, allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf(`Test\w+: %d, Test\d+: %d, Test\s+: %d`,
		len(wordChars), len(digits), len(whitespace))
}

// TestRegexPatterns_Alternation tests alternation (|)
func TestRegexPatterns_Alternation(t *testing.T) {
	code := `package main

func TestFunction() {}
func TestMethod() {}
func TestHelper() {}
func OtherFunc() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Match Function OR Method
	alternate := engine.SearchWithOptions("Test(Function|Method)", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Match Test followed by F or M
	charAlternate := engine.SearchWithOptions("Test[FM]", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Test(Function|Method): %d, Test[FM]: %d",
		len(alternate), len(charAlternate))
}

// TestRegexPatterns_Groups tests grouping and capturing
func TestRegexPatterns_Groups(t *testing.T) {
	code := `package main

func TestABC() {}
func TestXYZ() {}
func Test123() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Grouped pattern
	grouped := engine.SearchWithOptions("Test([A-Z]{3})", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Non-capturing group
	nonCapturing := engine.SearchWithOptions("Test(?:[A-Z]{3})", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Test([A-Z]{3}): %d, Test(?:[A-Z]{3}): %d",
		len(grouped), len(nonCapturing))
}

// TestRegexPatterns_Escaping tests special character escaping
func TestRegexPatterns_Escaping(t *testing.T) {
	code := `package main

func Test() {}
func Test.Method() {} // Invalid Go, but tests escaping
var pattern = "Test.*"
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Literal dot (escaped)
	literalDot := engine.SearchWithOptions(`Test\.`, allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Any character (unescaped dot)
	anyChar := engine.SearchWithOptions(`Test.`, allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf(`Test\.: %d, Test.: %d`, len(literalDot), len(anyChar))
}

// TestRegexPatterns_CaseInsensitive tests regex with case insensitivity
func TestRegexPatterns_CaseInsensitive(t *testing.T) {
	code := `package main

func TestFunction() {}
func testfunction() {}
func TESTFUNCTION() {}
func TestFunc() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Case-sensitive regex
	caseSensitive := engine.SearchWithOptions("Test[a-z]+", allFiles, types.SearchOptions{
		UseRegex:        true,
		CaseInsensitive: false,
	})

	// Case-insensitive regex
	caseInsensitive := engine.SearchWithOptions("test[a-z]+", allFiles, types.SearchOptions{
		UseRegex:        true,
		CaseInsensitive: true,
	})

	t.Logf("Case-sensitive: %d, Case-insensitive: %d",
		len(caseSensitive), len(caseInsensitive))
}

// TestRegexPatterns_WithSymbolTypes tests regex with symbol type filtering
func TestRegexPatterns_WithSymbolTypes(t *testing.T) {
	code := `package main

func Test123() {}
type Test456 struct {}
var Test789 = "test"
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Regex for functions only
	funcRegex := engine.SearchWithOptions(`Test\d+`, allFiles, types.SearchOptions{
		UseRegex:    true,
		SymbolTypes: []string{"function"},
	})

	// Regex for types only
	typeRegex := engine.SearchWithOptions(`Test\d+`, allFiles, types.SearchOptions{
		UseRegex:    true,
		SymbolTypes: []string{"class"},
	})

	t.Logf("Functions with regex: %d, Types with regex: %d",
		len(funcRegex), len(typeRegex))
}

// TestRegexPatterns_ComplexPatterns tests complex real-world regex patterns
func TestRegexPatterns_ComplexPatterns(t *testing.T) {
	code := `package main

func HandleHTTPRequest() {}
func handleHttpRequest() {}
func ProcessHTMLData() {}
func parseXMLFile() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Match HTTP/HTML/XML
	acronyms := engine.SearchWithOptions("(HTTP|HTML|XML)", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Match camelCase with acronyms
	camelCase := engine.SearchWithOptions("[a-z]+[A-Z]{2,}", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Acronyms: %d, CamelCase with acronyms: %d",
		len(acronyms), len(camelCase))
}

// TestRegexPatterns_EmptyAndInvalid tests empty and invalid regex patterns
func TestRegexPatterns_EmptyAndInvalid(t *testing.T) {
	code := `package main

func TestFunc() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Empty pattern
	empty := engine.SearchWithOptions("", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Invalid regex (unmatched bracket)
	invalid := engine.SearchWithOptions("[Test", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Empty pattern: %d, Invalid regex: %d", len(empty), len(invalid))
}

// TestRegexPatterns_WithDeclarationOnly tests regex with declaration filtering
func TestRegexPatterns_WithDeclarationOnly(t *testing.T) {
	code := `package main

func Process123() {
	Process456()
}

func Process456() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Regex for all Process functions
	allProcess := engine.SearchWithOptions(`Process\d+`, allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Regex for declarations only
	declOnly := engine.SearchWithOptions(`Process\d+`, allFiles, types.SearchOptions{
		UseRegex:        true,
		DeclarationOnly: true,
	})

	t.Logf("All Process: %d, Declarations only: %d",
		len(allProcess), len(declOnly))
}

// TestRegexPatterns_MultilinePatterns tests patterns that might span lines
func TestRegexPatterns_MultilinePatterns(t *testing.T) {
	code := `package main

func TestMultiline(
	param1 string,
	param2 int,
) error {
	return nil
}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Simple pattern
	simple := engine.SearchWithOptions("TestMultiline", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Pattern with whitespace
	withSpace := engine.SearchWithOptions(`TestMultiline\s*\(`, allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Simple: %d, With whitespace: %d", len(simple), len(withSpace))
}

// TestRegexPatterns_UnicodeSupport tests regex with unicode characters
func TestRegexPatterns_UnicodeSupport(t *testing.T) {
	code := `package main

func TestFunction() {}
func Test函数() {} // Chinese
func TestФункция() {} // Russian
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// ASCII only
	asciiOnly := engine.SearchWithOptions("Test[a-zA-Z]+", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Any characters after Test
	anyChars := engine.SearchWithOptions("Test.+", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("ASCII only: %d, Any chars: %d", len(asciiOnly), len(anyChars))
}

// TestRegexPatterns_PerformanceComparison tests literal vs regex performance
func TestRegexPatterns_PerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create larger test file
	code := `package main

func TestFunction1() {}
func TestFunction2() {}
func TestFunction3() {}
func TestFunction4() {}
func TestFunction5() {}
func HelperFunction() {}
func UtilityFunction() {}
`

	indexer, engine, _ := setupTestProject(t, code, "test.go")
	defer indexer.Close()

	allFiles := indexer.GetAllFileIDs()

	// Literal search
	literal := engine.SearchWithOptions("TestFunction", allFiles, types.SearchOptions{
		UseRegex: false,
	})

	// Regex search (same pattern)
	regex := engine.SearchWithOptions("TestFunction", allFiles, types.SearchOptions{
		UseRegex: true,
	})

	// Complex regex
	complexRegex := engine.SearchWithOptions(`Test.*\d+`, allFiles, types.SearchOptions{
		UseRegex: true,
	})

	t.Logf("Literal: %d, Regex (same): %d, Complex regex: %d",
		len(literal), len(regex), len(complexRegex))
}
