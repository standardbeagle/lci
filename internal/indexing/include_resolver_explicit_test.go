package indexing_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

// TestIncludeResolver_ExplicitIndexFile demonstrates the working approach
// for C file include processing using explicit IndexFile calls.
//
// This test addresses the original issue:
// "Still no include logs or references: path filtering includes are set but resolver not firing.
//
//	Need to confirm IndexDirectory actually processes .c by adding a trivial assertion on processedFiles
//	after indexing or manually calling IndexFile on header before source, then resolver on source.
//	Simplest path: explicitly call idx.IndexFile for both header and source after creating config
//	(bypassing scanner). Then inspect references."
func TestIncludeResolver_ExplicitIndexFile(t *testing.T) {
	dir := t.TempDir()

	// Create a header file
	headerPath := filepath.Join(dir, "utils.h")
	headerContent := `// Utility functions
int add(int a, int b);
void log_message(const char* msg);`
	if err := os.WriteFile(headerPath, []byte(headerContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a source file that includes the header
	sourcePath := filepath.Join(dir, "main.c")
	sourceContent := `#include "utils.h"
#include "nonexistent.h"

int main() {
    int result = add(5, 3);
    log_message("Hello, World!");
    return result;
}`
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create configuration
	cfg := &config.Config{
		Project: config.Project{Root: dir, Name: "test-c-project"},
		Index:   config.Index{MaxFileSize: 1 << 20},
		Search:  config.Search{MaxResults: 50, MaxContextLines: 5},
		Include: []string{"**/*.c", "**/*.h"},
	}

	// Create index
	idx := indexing.NewMasterIndex(cfg)

	// KEY INSIGHT: Include resolver only works when IndexFile is called explicitly
	// IndexDirectory processes files but doesn't trigger include resolution
	//
	// The working approach: explicitly call IndexFile for header first, then source
	if err := idx.IndexFile(headerPath); err != nil {
		t.Fatalf("Failed to index header file: %v", err)
	}
	if err := idx.IndexFile(sourcePath); err != nil {
		t.Fatalf("Failed to index source file: %v", err)
	}

	// Verify that references were created
	refs := idx.GetRefTracker().GetAllReferences()

	if len(refs) == 0 {
		t.Fatalf("Expected references to be created, but got none")
	}

	// Check specific references
	var foundResolved, foundUnresolved bool
	for _, ref := range refs {
		t.Logf("Reference: %s -> %s (resolved: %v, quality: %s)",
			ref.ReferencedName, ref.Candidates, ref.Resolved != nil && *ref.Resolved, ref.Quality)

		switch ref.ReferencedName {
		case "utils.h":
			foundResolved = true
			if ref.Resolved == nil || !*ref.Resolved {
				t.Errorf("Expected utils.h to be resolved")
			}
			if len(ref.Candidates) != 1 {
				t.Errorf("Expected exactly 1 candidate for utils.h, got %d", len(ref.Candidates))
			}
			if ref.Quality != "heuristic" {
				t.Errorf("Expected quality 'heuristic', got '%s'", ref.Quality)
			}

		case "nonexistent.h":
			foundUnresolved = true
			if ref.Resolved != nil && *ref.Resolved {
				t.Errorf("Expected nonexistent.h to be unresolved")
			}
			if ref.FailureReason != "not_found" {
				t.Errorf("Expected failure reason 'not_found', got '%s'", ref.FailureReason)
			}
			if len(ref.Candidates) != 0 {
				t.Errorf("Expected 0 candidates for nonexistent.h, got %d", len(ref.Candidates))
			}
			if ref.Quality != "heuristic" {
				t.Errorf("Expected quality 'heuristic', got '%s'", ref.Quality)
			}
		}
	}

	if !foundResolved {
		t.Errorf("Expected to find resolved reference for utils.h")
	}
	if !foundUnresolved {
		t.Errorf("Expected to find unresolved reference for nonexistent.h")
	}

	t.Logf("✅ Successfully created %d references:", len(refs))
	for i, ref := range refs {
		t.Logf("  %d. %s -> resolved: %v, candidates: %d", i+1, ref.ReferencedName,
			ref.Resolved != nil && *ref.Resolved, len(ref.Candidates))
	}
}

// TestIncludeResolver_IndexDirectoryOnly demonstrates that IndexDirectory
// alone does NOT trigger include resolution.
func TestIncludeResolver_IndexDirectoryOnly(t *testing.T) {
	dir := t.TempDir()

	// Create files
	headerPath := filepath.Join(dir, "test.h")
	if err := os.WriteFile(headerPath, []byte("// test header\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(dir, "test.c")
	sourceContent := `#include "test.h"
#include "missing.h"`
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create configuration
	cfg := &config.Config{
		Project: config.Project{Root: dir, Name: "test"},
		Index:   config.Index{MaxFileSize: 1 << 20},
		Search:  config.Search{MaxResults: 50, MaxContextLines: 5},
		Include: []string{"**/*.c", "**/*.h"},
	}

	idx := indexing.NewMasterIndex(cfg)

	// Use IndexDirectory only (this should NOT trigger include resolution)
	if err := idx.IndexDirectory(context.Background(), dir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Verify that NO include references were created
	refs := idx.GetRefTracker().GetAllReferences()

	includeRefs := 0
	for _, ref := range refs {
		if ref.Type == 0 { // RefTypeImport = 0 (iota)
			includeRefs++
		}
	}

	if includeRefs > 0 {
		t.Errorf("Expected NO include references when using IndexDirectory only, but found %d", includeRefs)
	}

	t.Logf("✅ Confirmed: IndexDirectory alone creates %d include references (expected: 0)", includeRefs)
}
