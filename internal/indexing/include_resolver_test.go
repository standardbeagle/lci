package indexing_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/testhelpers"
)

func TestIncludeResolver_ResolvedAndNotFound(t *testing.T) {
	dir := t.TempDir()
	// target header
	headerPath := filepath.Join(dir, "target.h")
	if err := os.WriteFile(headerPath, []byte("// header\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// source file referencing existing and missing header
	source := `#include "target.h"
#include "missing.h"
int main(){return 0;}`
	srcPath := filepath.Join(dir, "main.c")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := createTestConfig(dir)
	idx := indexing.NewMasterIndex(cfg)

	// NOTE: Include resolver only works when IndexFile is called explicitly
	// IndexDirectory doesn't trigger include resolution
	if err := idx.IndexFile(headerPath); err != nil {
		t.Fatalf("index header: %v", err)
	}
	if err := idx.IndexFile(srcPath); err != nil {
		t.Fatalf("index source: %v", err)
	}

	refs := idx.GetRefTracker().GetAllReferences()
	var foundExisting, foundMissing bool
	for _, r := range refs {
		if r.ReferencedName == "target.h" {
			foundExisting = true
			if r.Resolved == nil || !*r.Resolved {
				t.Errorf("expected resolved for target.h")
			}
		}
		if r.ReferencedName == "missing.h" {
			foundMissing = true
			if r.Resolved == nil || *r.Resolved {
				t.Errorf("expected unresolved for missing.h")
			}
			if r.FailureReason != "not_found" {
				t.Errorf("expected failure_reason not_found")
			}
		}
	}
	if !foundExisting {
		t.Errorf("expected reference for target.h")
	}
	if !foundMissing {
		t.Errorf("expected reference for missing.h")
	}
}

// collectRefs helper
// removed helper

func createTestConfig(rootDir string) *config.Config {
	return testhelpers.NewTestConfigBuilder(rootDir).
		WithIncludePatterns("**/*.c", "**/*.h").
		Build()
}
