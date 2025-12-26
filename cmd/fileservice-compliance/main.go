package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/standardbeagle/lci/internal/testing"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: fileservice-compliance <directory>")
		fmt.Println("Example: fileservice-compliance .")
		os.Exit(1)
	}

	rootDir := os.Args[1]

	// Resolve to absolute path
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔍 Checking FileService abstraction compliance in: %s\n\n", absPath)

	checker := testing.NewFileServiceComplianceChecker()

	// Include all files for comprehensive check
	checker.IgnoreTestFiles = false
	checker.IgnoreToolFiles = false

	err = checker.CheckDirectory(absPath)
	if err != nil {
		fmt.Printf("❌ Error checking compliance: %v\n", err)
		os.Exit(1)
	}

	// Print results
	checker.PrintViolations()

	// Set appropriate exit code
	if checker.HasErrors() {
		fmt.Printf("\n❌ Critical FileService violations found - architecture compromised\n")
		os.Exit(1)
	} else if checker.HasWarnings() {
		fmt.Printf("\n⚠️  FileService warnings found - improvements recommended\n")
		os.Exit(2)
	} else {
		fmt.Printf("\n✅ Perfect FileService abstraction compliance!\n")
		os.Exit(0)
	}
}