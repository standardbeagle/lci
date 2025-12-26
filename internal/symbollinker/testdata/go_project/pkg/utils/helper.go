package utils

import (
	"fmt"
	"strings"

	// Relative import within the module
	"github.com/example/testproject/pkg/utils/subpkg"
)

// Helper is an exported function
func Helper() {
	fmt.Println("Helper function")
	subpkg.SubHelper()
}

// ProcessString processes a string
func ProcessString(s string) string {
	return strings.ToUpper(s)
}
