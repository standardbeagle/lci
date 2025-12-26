package search

import (
	"runtime"
	"testing"

	"github.com/standardbeagle/lci/internal/core"
)

// runIsolatedTest provides simple test isolation without external dependencies
func runIsolatedTest(t *testing.T, testName string, testFunc func(*core.FileContentStore)) {
	// Force garbage collection to clean up any previous test state
	runtime.GC()
	runtime.GC()

	// Create a fresh file store for this test
	fileStore := core.NewFileContentStore()

	// Run the test function with the isolated file store
	testFunc(fileStore)

	// Cleanup after test
	runtime.GC()
	runtime.GC()
}

// createIsolatedFileContent generates test content with isolated variables
func createIsolatedFileContent(packageName string, elements []string) []byte {
	content := "package " + packageName + "\n\n"
	for _, element := range elements {
		content += element + "\n"
	}
	return []byte(content)
}
