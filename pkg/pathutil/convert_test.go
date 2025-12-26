package pathutil

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

func TestToRelative(t *testing.T) {
	tests := []struct {
		name     string
		absPath  string
		rootDir  string
		expected string
	}{
		{
			name:     "simple relative path",
			absPath:  "/home/user/project/src/main.go",
			rootDir:  "/home/user/project",
			expected: "src/main.go",
		},
		{
			name:     "nested relative path",
			absPath:  "/home/user/project/internal/core/search.go",
			rootDir:  "/home/user/project",
			expected: "internal/core/search.go",
		},
		{
			name:     "root level file",
			absPath:  "/home/user/project/README.md",
			rootDir:  "/home/user/project",
			expected: "README.md",
		},
		{
			name:     "same directory",
			absPath:  "/home/user/project",
			rootDir:  "/home/user/project",
			expected: ".",
		},
		{
			name:     "already relative path",
			absPath:  "src/main.go",
			rootDir:  "/home/user/project",
			expected: "src/main.go", // Should return as-is if already relative
		},
		{
			name:     "path outside root - fallback to absolute",
			absPath:  "/other/location/file.go",
			rootDir:  "/home/user/project",
			expected: "/other/location/file.go", // Should return absolute if outside root
		},
		{
			name:     "empty root directory",
			absPath:  "/home/user/project/file.go",
			rootDir:  "",
			expected: "/home/user/project/file.go", // Fallback to absolute
		},
		{
			name:     "empty absolute path",
			absPath:  "",
			rootDir:  "/home/user/project",
			expected: "", // Empty stays empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToRelative(tt.absPath, tt.rootDir)

			// Normalize separators for cross-platform testing
			if runtime.GOOS == "windows" {
				result = filepath.ToSlash(result)
				expected := filepath.ToSlash(tt.expected)
				if result != expected {
					t.Errorf("ToRelative() = %v, want %v", result, expected)
				}
			} else {
				if result != tt.expected {
					t.Errorf("ToRelative() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestToRelativeGrepResults(t *testing.T) {
	rootDir := "/home/user/project"

	input := []searchtypes.GrepResult{
		{
			FileID: types.FileID(1),
			Path:   "/home/user/project/src/main.go",
			Line:   10,
			Column: 5,
			Match:  "foo",
		},
		{
			FileID: types.FileID(2),
			Path:   "/home/user/project/internal/core/search.go",
			Line:   42,
			Column: 12,
			Match:  "bar",
		},
		{
			FileID: types.FileID(3),
			Path:   "/home/user/project/README.md",
			Line:   1,
			Column: 1,
			Match:  "baz",
		},
	}

	results := ToRelativeGrepResults(input, rootDir)

	expected := []string{
		"src/main.go",
		"internal/core/search.go",
		"README.md",
	}

	if len(results) != len(expected) {
		t.Fatalf("Expected %d results, got %d", len(expected), len(results))
	}

	for i, result := range results {
		// Normalize for cross-platform
		gotPath := result.Path
		wantPath := expected[i]
		if runtime.GOOS == "windows" {
			gotPath = filepath.ToSlash(gotPath)
			wantPath = filepath.ToSlash(wantPath)
		}

		if gotPath != wantPath {
			t.Errorf("Result %d: Path = %v, want %v", i, gotPath, wantPath)
		}

		// Verify other fields are unchanged
		if result.FileID != input[i].FileID {
			t.Errorf("Result %d: FileID changed", i)
		}
		if result.Line != input[i].Line {
			t.Errorf("Result %d: Line changed", i)
		}
		if result.Column != input[i].Column {
			t.Errorf("Result %d: Column changed", i)
		}
		if result.Match != input[i].Match {
			t.Errorf("Result %d: Match changed", i)
		}
	}
}

func TestToRelativeStandardResults(t *testing.T) {
	rootDir := "/home/user/project"

	input := []searchtypes.StandardResult{
		{
			Result: searchtypes.GrepResult{
				FileID: types.FileID(1),
				Path:   "/home/user/project/src/main.go",
				Line:   10,
				Column: 5,
				Match:  "foo",
			},
			ObjectID: "abc123",
		},
		{
			Result: searchtypes.GrepResult{
				FileID: types.FileID(2),
				Path:   "/home/user/project/internal/core/search.go",
				Line:   42,
				Column: 12,
				Match:  "bar",
			},
			ObjectID: "def456",
		},
	}

	results := ToRelativeStandardResults(input, rootDir)

	expected := []string{
		"src/main.go",
		"internal/core/search.go",
	}

	if len(results) != len(expected) {
		t.Fatalf("Expected %d results, got %d", len(expected), len(results))
	}

	for i, result := range results {
		// Normalize for cross-platform
		gotPath := result.Result.Path
		wantPath := expected[i]
		if runtime.GOOS == "windows" {
			gotPath = filepath.ToSlash(gotPath)
			wantPath = filepath.ToSlash(wantPath)
		}

		if gotPath != wantPath {
			t.Errorf("Result %d: Path = %v, want %v", i, gotPath, wantPath)
		}

		// Verify ObjectID is unchanged
		if result.ObjectID != input[i].ObjectID {
			t.Errorf("Result %d: ObjectID changed", i)
		}
	}
}

func TestToRelativeEmptySlice(t *testing.T) {
	rootDir := "/home/user/project"

	// Test empty GrepResults
	emptyGrep := []searchtypes.GrepResult{}
	resultGrep := ToRelativeGrepResults(emptyGrep, rootDir)
	if len(resultGrep) != 0 {
		t.Errorf("Expected empty slice for GrepResults, got %d elements", len(resultGrep))
	}

	// Test empty StandardResults
	emptyStandard := []searchtypes.StandardResult{}
	resultStandard := ToRelativeStandardResults(emptyStandard, rootDir)
	if len(resultStandard) != 0 {
		t.Errorf("Expected empty slice for StandardResults, got %d elements", len(resultStandard))
	}
}

func TestToRelativePreservesOtherFields(t *testing.T) {
	rootDir := "/home/user/project"

	input := []searchtypes.GrepResult{
		{
			FileID: types.FileID(42),
			Path:   "/home/user/project/test.go",
			Line:   100,
			Column: 25,
			Match:  "test match",
			Context: searchtypes.ExtractedContext{
				Lines:      []string{"line1", "line2", "line3"},
				StartLine:  98,
				EndLine:    102,
				MatchCount: 3,
			},
			Score:          0.95,
			FileMatchCount: 5,
		},
	}

	results := ToRelativeGrepResults(input, rootDir)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	r := results[0]

	// Check all fields are preserved
	if r.FileID != input[0].FileID {
		t.Errorf("FileID not preserved: got %v, want %v", r.FileID, input[0].FileID)
	}
	if r.Line != input[0].Line {
		t.Errorf("Line not preserved: got %v, want %v", r.Line, input[0].Line)
	}
	if r.Column != input[0].Column {
		t.Errorf("Column not preserved: got %v, want %v", r.Column, input[0].Column)
	}
	if r.Match != input[0].Match {
		t.Errorf("Match not preserved: got %v, want %v", r.Match, input[0].Match)
	}
	if r.Score != input[0].Score {
		t.Errorf("Score not preserved: got %v, want %v", r.Score, input[0].Score)
	}
	if r.FileMatchCount != input[0].FileMatchCount {
		t.Errorf("FileMatchCount not preserved: got %v, want %v", r.FileMatchCount, input[0].FileMatchCount)
	}
	if len(r.Context.Lines) != len(input[0].Context.Lines) {
		t.Errorf("Context.Lines not preserved: got %v, want %v", r.Context.Lines, input[0].Context.Lines)
	}
	if r.Context.StartLine != input[0].Context.StartLine {
		t.Errorf("Context.StartLine not preserved: got %v, want %v", r.Context.StartLine, input[0].Context.StartLine)
	}
	if r.Context.MatchCount != input[0].Context.MatchCount {
		t.Errorf("Context.MatchCount not preserved: got %v, want %v", r.Context.MatchCount, input[0].Context.MatchCount)
	}
}
