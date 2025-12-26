package core

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain ensures no goroutines leak in any test in the core package.
// This is critical since core components like CallGraph, SymbolIndex, and
// ReferenceTracker are designed for concurrent access with lock-free patterns.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Ignore known background goroutines
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("sync.runtime_Semacquire"),
	)
}
