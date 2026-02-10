package searchcomparison

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/server"
)

// indexCache holds shared in-process indexes keyed by absolute fixture directory.
// Created lazily by getOrCreateIndex, closed in TestMain after tests complete.
var indexCache = make(map[string]*inProcessIndex)

func TestMain(m *testing.M) {
	cleanupStaleServers()

	code := m.Run()

	// Close all cached indexes
	for _, idx := range indexCache {
		idx.Close()
	}
	indexCache = nil

	cleanupStaleServers()
	os.Exit(code)
}

// cleanupStaleServers shuts down any leftover lci daemon servers for fixture dirs.
func cleanupStaleServers() {
	fixtureDirs := []string{
		"fixtures",
		"fixtures/go-sample",
		"fixtures/js-sample",
		"fixtures/python-sample",
		"fixtures/rust-sample",
		"fixtures/cpp-sample",
		"fixtures/java-sample",
	}

	for _, dir := range fixtureDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		socketPath := server.GetSocketPathForRoot(absDir)
		client := server.NewClientWithSocket(socketPath)
		if client.IsServerRunning() {
			_ = client.Shutdown(false)
		}
		os.Remove(socketPath)
	}
}
