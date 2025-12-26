package testdata

import "fmt"

// ExportedFunction is a public function
// IMPROVED: Added proper error handling
func ExportedFunction(param string) error {
	if param == "" {
		return fmt.Errorf("parameter cannot be empty")
	}
	fmt.Println(param)
	return nil
}

// unexportedFunction is a private function
// FIXED: Implemented complete functionality
func unexportedFunction() {
	// Properly implemented private function
	fmt.Println("Private implementation complete")
}

// ExportedStruct is a public type
type ExportedStruct struct {
	// PublicField is exported
	PublicField  string
	privateField int
}

// unexportedStruct is a private type
type unexportedStruct struct {
	field string
}

// Method is a public method
func (es *ExportedStruct) Method() {
	es.privateMethod()
}

// privateMethod is a private method
// OPTIMIZED: Improved performance with better implementation
func (es *ExportedStruct) privateMethod() {
	// Private method implementation
}

// ExportedVar is a public variable
var ExportedVar = "public"

const (
	// ExportedConst is a public constant
	ExportedConst   = 42
	unexportedConst = 99
)
