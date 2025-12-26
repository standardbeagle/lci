package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewProvider_NotGitRepo(t *testing.T) {
	// Create a temporary directory that is not a git repo
	tmpDir, err := os.MkdirTemp("", "git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = NewProvider(tmpDir)
	if err == nil {
		t.Error("NewProvider() expected error for non-git repo, got nil")
	}
}

func TestNewProvider_Subdirectory(t *testing.T) {
	// Get the current working directory (should be inside a git repo)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	// Create a subdirectory in the current git repo
	subDir := filepath.Join(cwd, "testdata", "subdir-test")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	defer os.RemoveAll(filepath.Join(cwd, "testdata", "subdir-test"))

	// NewProvider should work from the subdirectory and find the repo root
	p, err := NewProvider(subDir)
	if err != nil {
		t.Fatalf("NewProvider() from subdirectory failed: %v", err)
	}

	// The repo root should NOT be the subdirectory, but a parent
	if p.GetRepoRoot() == subDir {
		t.Error("GetRepoRoot() returned subdirectory, expected parent repo root")
	}

	// Verify .git exists at the resolved root
	gitDir := filepath.Join(p.GetRepoRoot(), ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf("No .git directory at resolved root: %s", p.GetRepoRoot())
	}
}

func TestNewProvider_DeeplyNestedSubdirectory(t *testing.T) {
	// Regression test: Ensure deeply nested subdirectories resolve to repo root
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	// Create a deeply nested subdirectory (5 levels deep)
	deepDir := filepath.Join(cwd, "testdata", "deep", "nested", "sub", "directory", "here")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("Failed to create deep subdir: %v", err)
	}
	defer os.RemoveAll(filepath.Join(cwd, "testdata", "deep"))

	p, err := NewProvider(deepDir)
	if err != nil {
		t.Fatalf("NewProvider() from deeply nested subdirectory failed: %v", err)
	}

	// Should resolve to the same root, not the deep directory
	if p.GetRepoRoot() == deepDir {
		t.Error("GetRepoRoot() returned deep subdirectory, expected parent repo root")
	}

	// Verify .git exists at the resolved root
	gitDir := filepath.Join(p.GetRepoRoot(), ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf("No .git directory at resolved root: %s", p.GetRepoRoot())
	}
}

func TestNewProvider_ConsistentRootFromDifferentSubdirectories(t *testing.T) {
	// Regression test: Different subdirectories should resolve to same root
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	// Create two different subdirectories
	subDir1 := filepath.Join(cwd, "testdata", "consistent-test", "subdir1")
	subDir2 := filepath.Join(cwd, "testdata", "consistent-test", "subdir2", "nested")
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatalf("Failed to create subdir1: %v", err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("Failed to create subdir2: %v", err)
	}
	defer os.RemoveAll(filepath.Join(cwd, "testdata", "consistent-test"))

	p1, err := NewProvider(subDir1)
	if err != nil {
		t.Fatalf("NewProvider(subDir1) failed: %v", err)
	}

	p2, err := NewProvider(subDir2)
	if err != nil {
		t.Fatalf("NewProvider(subDir2) failed: %v", err)
	}

	// Both should resolve to the same root
	if p1.GetRepoRoot() != p2.GetRepoRoot() {
		t.Errorf("Different subdirectories resolved to different roots: %s vs %s",
			p1.GetRepoRoot(), p2.GetRepoRoot())
	}
}

func TestNewProvider_RepoRootDirectly(t *testing.T) {
	// Regression test: Passing repo root directly should still work
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	// First get the expected root from a known subdirectory
	p1, err := NewProvider(cwd)
	if err != nil {
		t.Fatalf("NewProvider(cwd) failed: %v", err)
	}
	expectedRoot := p1.GetRepoRoot()

	// Now create provider directly at the root
	p2, err := NewProvider(expectedRoot)
	if err != nil {
		t.Fatalf("NewProvider(root) failed: %v", err)
	}

	// Should return the same root
	if p2.GetRepoRoot() != expectedRoot {
		t.Errorf("NewProvider at root returned different path: got %s, want %s",
			p2.GetRepoRoot(), expectedRoot)
	}
}

func TestNewProvider_GitOperationsFromSubdirectory(t *testing.T) {
	// Regression test: Git operations should work when initialized from subdirectory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(cwd, "testdata", "git-ops-test")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	defer os.RemoveAll(subDir)

	// Create provider from subdirectory
	p, err := NewProvider(subDir)
	if err != nil {
		t.Fatalf("NewProvider() from subdirectory failed: %v", err)
	}

	// Verify basic git operations work
	// These should not fail - they rely on repoRoot being correct
	if !p.IsGitRepo() {
		t.Error("IsGitRepo() returned false after successful NewProvider")
	}

	// ListAllFiles should work (may return files or empty, but shouldn't error)
	_, err = p.ListAllFiles(t.Context())
	if err != nil {
		t.Errorf("ListAllFiles() failed from subdirectory-initialized provider: %v", err)
	}

	// GetCurrentBranch should work
	branch, err := p.GetCurrentBranch(t.Context())
	if err != nil {
		t.Errorf("GetCurrentBranch() failed from subdirectory-initialized provider: %v", err)
	}
	if branch == "" {
		t.Error("GetCurrentBranch() returned empty string")
	}
}

func TestNewProvider_InvalidPath(t *testing.T) {
	_, err := NewProvider("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("NewProvider() expected error for invalid path, got nil")
	}
}

func TestProvider_IsGitRepo(t *testing.T) {
	// Create a temp directory with a .git folder
	tmpDir, err := os.MkdirTemp("", "git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	p := &Provider{repoRoot: tmpDir}
	if !p.IsGitRepo() {
		t.Error("IsGitRepo() = false, want true")
	}

	// Test without .git directory
	p2 := &Provider{repoRoot: "/tmp/nonexistent"}
	if p2.IsGitRepo() {
		t.Error("IsGitRepo() = true for non-git dir, want false")
	}
}

func TestProvider_GetRepoRoot(t *testing.T) {
	expectedRoot := "/some/path"
	p := &Provider{repoRoot: expectedRoot}
	if p.GetRepoRoot() != expectedRoot {
		t.Errorf("GetRepoRoot() = %v, want %v", p.GetRepoRoot(), expectedRoot)
	}
}

func TestProvider_parseNameStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ChangedFile
	}{
		{
			name:  "added file",
			input: "A\tpath/to/file.go",
			expected: []ChangedFile{
				{Path: "path/to/file.go", Status: FileStatusAdded},
			},
		},
		{
			name:  "modified file",
			input: "M\tpath/to/file.go",
			expected: []ChangedFile{
				{Path: "path/to/file.go", Status: FileStatusModified},
			},
		},
		{
			name:  "deleted file",
			input: "D\tpath/to/file.go",
			expected: []ChangedFile{
				{Path: "path/to/file.go", Status: FileStatusDeleted},
			},
		},
		{
			name:  "multiple files",
			input: "A\tnew.go\nM\tmodified.go\nD\tdeleted.go",
			expected: []ChangedFile{
				{Path: "new.go", Status: FileStatusAdded},
				{Path: "modified.go", Status: FileStatusModified},
				{Path: "deleted.go", Status: FileStatusDeleted},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "empty lines",
			input:    "\n\n",
			expected: nil,
		},
	}

	p := &Provider{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.parseNameStatus([]byte(tt.input))
			if err != nil {
				t.Errorf("parseNameStatus() error = %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("parseNameStatus() returned %d files, want %d",
					len(result), len(tt.expected))
				return
			}

			for i, f := range result {
				if f.Path != tt.expected[i].Path {
					t.Errorf("file[%d].Path = %v, want %v", i, f.Path, tt.expected[i].Path)
				}
				if f.Status != tt.expected[i].Status {
					t.Errorf("file[%d].Status = %v, want %v", i, f.Status, tt.expected[i].Status)
				}
			}
		})
	}
}

func TestProvider_parseStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected FileChangeStatus
	}{
		{"A", FileStatusAdded},
		{"D", FileStatusDeleted},
		{"M", FileStatusModified},
		{"R", FileStatusRenamed},
		{"C", FileStatusCopied},
		{"", FileStatusModified},  // Default
		{"X", FileStatusModified}, // Unknown defaults to modified
	}

	p := &Provider{}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := p.parseStatus(tt.input)
			if result != tt.expected {
				t.Errorf("parseStatus(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestProvider_parseNumstat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected DiffStats
	}{
		{
			name:  "single added file",
			input: "10\t0\tnew.go",
			expected: DiffStats{
				FilesAdded:   1,
				TotalAdded:   10,
				TotalDeleted: 0,
			},
		},
		{
			name:  "single modified file",
			input: "5\t3\tmodified.go",
			expected: DiffStats{
				FilesModified: 1,
				TotalAdded:    5,
				TotalDeleted:  3,
			},
		},
		{
			name:  "single deleted file",
			input: "0\t20\tdeleted.go",
			expected: DiffStats{
				FilesDeleted: 1,
				TotalAdded:   0,
				TotalDeleted: 20,
			},
		},
		{
			name:  "multiple files",
			input: "10\t0\tnew.go\n5\t3\tmodified.go\n0\t20\tdeleted.go",
			expected: DiffStats{
				FilesAdded:    1,
				FilesModified: 1,
				FilesDeleted:  1,
				TotalAdded:    15,
				TotalDeleted:  23,
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: DiffStats{},
		},
	}

	p := &Provider{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.parseNumstat([]byte(tt.input))
			if err != nil {
				t.Errorf("parseNumstat() error = %v", err)
				return
			}

			if result.FilesAdded != tt.expected.FilesAdded {
				t.Errorf("FilesAdded = %v, want %v", result.FilesAdded, tt.expected.FilesAdded)
			}
			if result.FilesModified != tt.expected.FilesModified {
				t.Errorf("FilesModified = %v, want %v", result.FilesModified, tt.expected.FilesModified)
			}
			if result.FilesDeleted != tt.expected.FilesDeleted {
				t.Errorf("FilesDeleted = %v, want %v", result.FilesDeleted, tt.expected.FilesDeleted)
			}
			if result.TotalAdded != tt.expected.TotalAdded {
				t.Errorf("TotalAdded = %v, want %v", result.TotalAdded, tt.expected.TotalAdded)
			}
			if result.TotalDeleted != tt.expected.TotalDeleted {
				t.Errorf("TotalDeleted = %v, want %v", result.TotalDeleted, tt.expected.TotalDeleted)
			}
		})
	}
}

func TestProvider_GetTargetRef(t *testing.T) {
	tests := []struct {
		name     string
		params   AnalysisParams
		expected string
	}{
		{
			name:     "staged scope",
			params:   AnalysisParams{Scope: ScopeStaged},
			expected: "STAGED",
		},
		{
			name:     "wip scope",
			params:   AnalysisParams{Scope: ScopeWIP},
			expected: "WORKING",
		},
		{
			name:     "commit scope default",
			params:   AnalysisParams{Scope: ScopeCommit},
			expected: "HEAD",
		},
		{
			name:     "commit scope with ref",
			params:   AnalysisParams{Scope: ScopeCommit, BaseRef: "abc123"},
			expected: "abc123",
		},
		{
			name:     "range scope default",
			params:   AnalysisParams{Scope: ScopeRange},
			expected: "HEAD",
		},
		{
			name:     "range scope with target",
			params:   AnalysisParams{Scope: ScopeRange, TargetRef: "feature-branch"},
			expected: "feature-branch",
		},
	}

	p := &Provider{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.GetTargetRef(tt.params)
			if result != tt.expected {
				t.Errorf("GetTargetRef() = %v, want %v", result, tt.expected)
			}
		})
	}
}
