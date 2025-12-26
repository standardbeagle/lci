package fixtures

import (
	"fmt"
	"strings"
)

// SearchTestFiles provides realistic test data for comprehensive search testing
type SearchTestFiles struct {
	Name    string
	Content string
	Purpose string
}

// GetRegexPatternTestFile returns test code with various regex patterns
func GetRegexPatternTestFile() SearchTestFiles {
	return SearchTestFiles{
		Name:    "regex_patterns.go",
		Purpose: "Testing regex pattern matching",
		Content: `package main

import "regexp"

func TestRegexPatterns() {
	// Email validation
	emailPattern := "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"

	// Phone number (XXX-XXX-XXXX)
	phonePattern := "[0-9]{3}-[0-9]{3}-[0-9]{4}"

	// URL matching
	urlPattern := "^https?://"

	// Word boundaries
	wordPattern := "\\bword\\b"

	// Character classes
	digitClass := "[0-9]"
	alphaClass := "[a-zA-Z]"
	whitespaceClass := "[\\s]"

	// Anchors
	startAnchor := "^test"
	endAnchor := "test$"

	// Alternation
	alternation := "(foo|bar|baz)"

	// Quantifiers
	zeroOrMore := "a*"
	oneOrMore := "a+"
	optional := "a?"
	exact := "a{3}"
	range := "a{2,4}"
}`,
	}
}

// GetCaseSensitivityTestFile returns test code for case sensitivity testing
func GetCaseSensitivityTestFile() SearchTestFiles {
	return SearchTestFiles{
		Name:    "case_sensitivity.go",
		Purpose: "Testing case-sensitive and case-insensitive search",
		Content: `package main

import "strings"

// UPPERCASE constants
const (
	CONSTANT_VALUE = 42
	DATABASE_HOST = "localhost"
	API_KEY_SECRET = "secret123"
)

// lowercase variables
var (
	packageName = "mypackage"
	configFile = "config.yaml"
	dataSource = "database"
)

// CamelCase structs
type UserProfile struct {
	FirstName string
	LastName  string
}

type APIResponse struct {
	StatusCode int
	Message    string
}

// mixedCase functions
func ProcessData() {
	userData := GetUserData()
	processUserData(userData)
}

func GetUserData() map[string]interface{} {
	return map[string]interface{}{}
}

func processUserData(data map[string]interface{}) {
	// Implementation
}

// Custom case patterns
func CustomCaseTest() {
	snake_case_var := "value"
	kebab_case_constant := "CONSTANT"
	camelCaseFunction := "function"
	PascalCaseType := "Type"
}`,
	}
}

// GetSymbolTypeTestFile returns test code with various symbol types
func GetSymbolTypeTestFile() SearchTestFiles {
	return SearchTestFiles{
		Name:    "symbol_types.go",
		Purpose: "Testing symbol type filtering (functions, types, constants, etc)",
		Content: `package main

// Types
type DataProcessor struct {
	name string
	data map[string]interface{}
}

type Handler interface {
	Handle(data interface{}) error
}

type Result struct {
	success bool
	value   interface{}
}

// Interfaces
type Reader interface {
	Read() ([]byte, error)
}

type Writer interface {
	Write([]byte) (int, error)
}

// Constants
const (
	MaxRetries = 3
	DefaultTimeout = 30
	VERSION = "1.0.0"
)

// Variables
var (
	globalCache map[string]interface{}
	logger interface{}
	isDebug = false
)

// Functions
func ProcessRequest(input string) (string, error) {
	return "processed", nil
}

func ValidateData(data interface{}) bool {
	return data != nil
}

func Initialize() error {
	return nil
}

// Methods
func (dp *DataProcessor) Process(input interface{}) interface{} {
	return input
}

func (h Handler) HandleRequest(data interface{}) {
	_ = h.Handle(data)
}

// Exported vs unexported
func ExportedFunction() {
	privateHelper()
}

func privateHelper() {
	// private implementation
}

// Anonymous functions/closures
func GetProcessor() func(string) string {
	return func(input string) string {
		return "processed: " + input
	}
}`,
	}
}

// GetNestedCodeStructure returns deeply nested code for complexity testing
func GetNestedCodeStructure() SearchTestFiles {
	return SearchTestFiles{
		Name:    "nested_structures.go",
		Purpose: "Testing search in deeply nested code structures",
		Content: `package main

import "fmt"

type OuterService struct {
	inner struct {
		data struct {
			value string
		}
	}
}

func ComplexFunction() {
	// Level 1
	if true {
		// Level 2
		for i := 0; i < 10; i++ {
			// Level 3
			switch i {
			case 0:
				// Level 4
				for j := 0; j < 5; j++ {
					// Level 5
					if j > 2 {
						// Level 6
						{
							// Level 7
							{
								// Level 8 - Very deeply nested
								value := "deeply nested data"
								processDeep(value)

								if true {
									// Level 9
									for k := range []int{1, 2, 3} {
										// Level 10
										_ = k
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func processDeep(value string) {
	fmt.Println(value)
}

type ComplexType struct {
	handlers []struct {
		name string
		fn   func(string) error
	}
}

func (ct *ComplexType) Execute(input string) error {
	for _, handler := range ct.handlers {
		if err := handler.fn(input); err != nil {
			return err
		}
	}
	return nil
}`,
	}
}

// GetDeclarationVsUsageTestFile tests declaration vs usage filtering
func GetDeclarationVsUsageTestFile() SearchTestFiles {
	return SearchTestFiles{
		Name:    "declaration_vs_usage.go",
		Purpose: "Testing declaration_only and usage_only filters",
		Content: `package main

// Declaration: ProcessData function is declared here
func ProcessData(input string) string {
	return "processed: " + input
}

// Usage 1: In function caller1
func caller1() string {
	result := ProcessData("data1")
	return result
}

// Usage 2: In function caller2
func caller2() {
	ProcessData("data2")
}

// Usage 3: In function caller3
func caller3() error {
	data, _ := readData()
	ProcessData(data)
	return nil
}

// Usage 4: In callback
func setupCallback() {
	callback := func(data string) {
		ProcessData(data)
	}
	executeCallback(callback)
}

// Usage 5: In deferred call
func withDefer() {
	defer func() {
		ProcessData("")
	}()
}

// Usage 6: In goroutine
func asyncProcess() {
	go func() {
		ProcessData("async")
	}()
}

func readData() (string, error) {
	return "data", nil
}

func executeCallback(fn func(string)) {
	fn("test")
}

// Another declaration for comparison
func HelperFunction() {
	// This function is declared here
}

func useHelper() {
	HelperFunction()
	HelperFunction()
}`,
	}
}

// GetExportedVsPrivateTestFile tests exported vs private filtering
func GetExportedVsPrivateTestFile() SearchTestFiles {
	return SearchTestFiles{
		Name:    "exported_vs_private.go",
		Purpose: "Testing exported_only filter",
		Content: `package mypackage

// Public API

// ExportedFunction is part of public API
func ExportedFunction() {
	internalHelper()
}

// ExportedType is part of public API
type ExportedType struct {
	PublicField string
	privateField string
}

// ExportedMethod is public
func (et *ExportedType) ExportedMethod() {
	et.privateHelper()
}

// ExportedConstant is public
const ExportedConstant = 42

// ExportedVariable is public
var ExportedVariable string

// Private Implementation

// privateFunction is internal only
func privateFunction() {
	// implementation
}

// privateType is internal only
type privateType struct {
	field string
}

// privateMethod is private
func (pt *privateType) privateMethod() {
	// implementation
}

// privateConstant is internal only
const privateConstant = 0

// privateVariable is internal only
var privateVariable string

// Private helpers that support public API

func internalHelper() {
	// Called by ExportedFunction
	privateFunction()
}

// privateHelper performs internal work
func (et *ExportedType) privateHelper() {
	// Internal work
	pt := privateType{}
	pt.privateMethod()
}`,
	}
}

// GenerateComplexProjectStructure creates test code representing a real project
func GenerateComplexProjectStructure(numPackages int) map[string]string {
	files := make(map[string]string)

	for p := 0; p < numPackages; p++ {
		pkgName := fmt.Sprintf("package%d", p)

		// Service file
		serviceFile := fmt.Sprintf(`package %s

type %sService struct {
	name string
}

func (s *%sService) Handle(input interface{}) (interface{}, error) {
	return s.process(input)
}

func (s *%sService) process(input interface{}) (interface{}, error) {
	// Processing logic
	return nil, nil
}

func New%sService() *%sService {
	return &%sService{name: "service%d"}
}`, pkgName, pkgName, pkgName, pkgName, pkgName, pkgName, pkgName, p)

		files[pkgName+"/service.go"] = serviceFile

		// Handler file
		handlerFile := fmt.Sprintf(`package %s

func HandleRequest(data interface{}) {
	svc := New%sService()
	svc.Handle(data)
}

func ValidateInput(input interface{}) bool {
	return input != nil
}`, pkgName, pkgName)

		files[pkgName+"/handler.go"] = handlerFile

		// Model file
		modelFile := fmt.Sprintf(`package %s

type Data%d struct {
	ID    int
	Value string
	Items []Item%d
}

type Item%d struct {
	Key   string
	Value interface{}
}`, pkgName, p, p, p)

		files[pkgName+"/models.go"] = modelFile
	}

	return files
}

// GetLargeFileWithManySymbols generates a large file with many symbols
func GetLargeFileWithManySymbols(symbolCount int) SearchTestFiles {
	var content strings.Builder
	content.WriteString("package main\n\n")
	content.WriteString("import \"fmt\"\n\n")

	// Generate types
	for i := 0; i < symbolCount/3; i++ {
		content.WriteString(fmt.Sprintf("type Type%d struct { field%d string }\n", i, i))
	}

	content.WriteString("\n")

	// Generate constants
	for i := 0; i < symbolCount/3; i++ {
		content.WriteString(fmt.Sprintf("const Constant%d = %d\n", i, i))
	}

	content.WriteString("\n")

	// Generate functions
	for i := 0; i < symbolCount/3; i++ {
		content.WriteString(fmt.Sprintf(`func Function%d() {
	// Implementation
	_ = Type%d{}
	_ = Constant%d
}

`, i, i, i))
	}

	return SearchTestFiles{
		Name:    fmt.Sprintf("large_%d_symbols.go", symbolCount),
		Purpose: fmt.Sprintf("Testing search in file with %d symbols", symbolCount),
		Content: content.String(),
	}
}

// GetMultiLanguageProject returns files in different languages
func GetMultiLanguageProject() map[string]SearchTestFiles {
	return map[string]SearchTestFiles{
		"service.go": {
			Name:    "service.go",
			Purpose: "Go service implementation",
			Content: `package main

func ProcessRequest(req interface{}) error {
	// Process implementation
	return nil
}`,
		},
		"utils.js": {
			Name:    "utils.js",
			Purpose: "JavaScript utilities",
			Content: `function ProcessRequest(request) {
	// JavaScript implementation
	console.log("Processing request");
	return request;
}

class RequestHandler {
	handle(request) {
		return ProcessRequest(request);
	}
}`,
		},
		"helpers.py": {
			Name:    "helpers.py",
			Purpose: "Python helpers",
			Content: `def ProcessRequest(request):
	"""Process a request in Python"""
	return {"status": "processed"}

class RequestProcessor:
	def process(self, request):
		return ProcessRequest(request)`,
		},
	}
}

// GetCommentTestFile returns code with various comment types
func GetCommentTestFile() SearchTestFiles {
	return SearchTestFiles{
		Name:    "comments.go",
		Purpose: "Testing exclude_comments filter",
		Content: `package main

import "fmt"

// Function documentation
// This function processes test data
func ProcessTestData() {
	// Line comment about test
	result := "test data"

	/* Multi-line comment
	   describing test functionality
	   and test patterns
	*/
	fmt.Println(result)

	/*
	Block comment with test
	information
	*/
}

// Test comment in function
func AnotherFunction() {
	// TODO: test this function
	// FIXME: test case is incomplete
	value := "test" // inline comment with test
}

func TestFunction() {
	// The word test appears many times
	// in this test function
	/* test in block comment */
}`,
	}
}
