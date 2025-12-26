//nolint:all // Test fixture file with intentionally verbose code
package fixtures

import (
	"fmt"
	"strings"
)

// GenerateLargeFile generates a large Go file with the specified number of functions
func GenerateLargeFile(numFunctions int) string {
	var sb strings.Builder

	// Package declaration
	sb.WriteString("package generated\n\n")
	sb.WriteString("import \"fmt\"\n\n")

	// Generate types
	for i := 0; i < numFunctions/5; i++ {
		sb.WriteString(fmt.Sprintf("// Type%d is a test type\n", i))
		sb.WriteString(fmt.Sprintf("type Type%d struct {\n", i))
		sb.WriteString("\tfield1 string\n")
		sb.WriteString("\tfield2 int\n")
		sb.WriteString("}\n\n")
	}

	// Generate functions
	for i := 0; i < numFunctions; i++ {
		// Add some variety
		if i%3 == 0 {
			// Method
			typeNum := i / 5
			sb.WriteString(fmt.Sprintf("// Method%d processes data\n", i))
			sb.WriteString(fmt.Sprintf("func (t *Type%d) Method%d(param1 string, param2 int) error {\n", typeNum, i))
			sb.WriteString("\t// Process the data\n")
			sb.WriteString("\tif param2 < 0 {\n")
			sb.WriteString("\t\treturn fmt.Errorf(\"negative value: %d\", param2)\n")
			sb.WriteString("\t}\n")
			sb.WriteString("\tt.field1 = param1\n")
			sb.WriteString("\tt.field2 = param2\n")
			sb.WriteString("\treturn nil\n")
			sb.WriteString("}\n\n")
		} else {
			// Regular function
			sb.WriteString(fmt.Sprintf("// Function%d performs operations\n", i))
			sb.WriteString(fmt.Sprintf("func Function%d(input string) string {\n", i))
			sb.WriteString(fmt.Sprintf("\t// Perform some operations\n"))
			sb.WriteString(fmt.Sprintf("\tresult := \"processed: \" + input\n"))
			sb.WriteString(fmt.Sprintf("\tif len(input) > 10 {\n"))
			sb.WriteString(fmt.Sprintf("\t\tresult = \"long input: \" + input[:10] + \"...\"\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
			sb.WriteString(fmt.Sprintf("\treturn result\n"))
			sb.WriteString("}\n\n")
		}
	}

	// Add a main function
	sb.WriteString("func main() {\n")
	sb.WriteString("\tfmt.Println(\"Generated test file\")\n")
	sb.WriteString("\t// Call some functions\n")
	for i := 0; i < 5 && i < numFunctions; i++ {
		sb.WriteString(fmt.Sprintf("\tFunction%d(\"test\")\n", i))
	}
	sb.WriteString("}\n")

	return sb.String()
}
