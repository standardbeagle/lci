package indexing

import (
	"testing"
)

// TestMain - main test entry point
// Goroutine leak detection has been moved to leak_test.go with build tag "leaktests"
// Run with: go test ./internal/indexing -tags=leaktests
func TestMain(m *testing.M) {
	m.Run()
}
