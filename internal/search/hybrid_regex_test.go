package search_test

import (
	"context"
	"os"
	"path/filepath"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
	"testing"
)

// TestHybridRegexLineAttribution ensures hybrid regex results have correct line/column (no zeros)
// and the line slice matches the reported Match text.
func TestHybridRegexLineAttribution(t *testing.T) {
	dir := t.TempDir()
	f1 := "alpha match_one here\nsecond line\nmatch_one again end\n"
	f2 := "preamble\nmatch_one start middle match_one end\ntrailer\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(f1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte(f2), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := createTestConfig(dir)
	idx := indexing.NewMasterIndex(cfg)
	if err := idx.IndexDirectory(context.Background(), dir); err != nil {
		t.Fatalf("index: %v", err)
	}

	eng := search.NewEngine(idx)
	files := idx.GetAllFileIDs()
	results := eng.SearchWithOptions("match_one", files, types.SearchOptions{UseRegex: true})
	if len(results) < 3 {
		t.Fatalf("expected >=3 matches, got %d", len(results))
	}
	for _, r := range results {
		if r.Line <= 0 {
			t.Fatalf("invalid line %d for %s", r.Line, r.Path)
		}
		if r.Column < 0 {
			t.Fatalf("invalid column %d for %s", r.Column, r.Path)
		}
		fi := idx.GetFileInfo(r.FileID)
		if fi == nil {
			t.Fatalf("nil file info for %s", r.Path)
		}
		lineBytes := getLine(fi.Content, r.Line)
		if r.Column+len(r.Match) > len(lineBytes) {
			t.Fatalf("span out of bounds: col=%d len=%d lineLen=%d", r.Column, len(r.Match), len(lineBytes))
		}
		if string(lineBytes[r.Column:r.Column+len(r.Match)]) != r.Match {
			t.Fatalf("mismatch slice vs match: %s vs %s", string(lineBytes[r.Column:r.Column+len(r.Match)]), r.Match)
		}
	}
}

// getLine simple helper (1-based line number)
func getLine(content []byte, line int) []byte {
	start := 0
	cur := 1
	for i, b := range content {
		if b == '\n' {
			if cur == line {
				return content[start:i]
			}
			cur++
			start = i + 1
		}
	}
	if cur == line {
		return content[start:]
	}
	return nil
}
