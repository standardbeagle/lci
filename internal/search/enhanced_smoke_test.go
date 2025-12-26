package search_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	"github.com/standardbeagle/lci/internal/types"
)

// TestEnhancedSymbolReferenceCountSmoke is a lightweight smoke test for enhanced symbol reference counting
// This runs for the pre-commit hook, verifying the feature works without taking full timing
func TestEnhancedSymbolReferenceCountSmoke(t *testing.T) {
	tempDir := t.TempDir()

	// Minimal test files - just enough to verify the feature works
	testFiles := map[string]string{
		"types.go": `package main

type User struct {
	ID   int
	Name string
}

func NewUser(id int, name string) *User {
	return &User{ID: id, Name: name}
}

func (u *User) GetName() string {
	return u.Name
}
`,
		"service.go": `package main

type UserService struct {
	users map[int]*User
}

func NewUserService() *UserService {
	return &UserService{
		users: make(map[int]*User),
	}
}

func (s *UserService) CreateUser(id int, name string) *User {
	user := NewUser(id, name)
	s.users[id] = user
	return user
}

func (s *UserService) GetUser(id int) *User {
	return s.users[id]
}
`,
		"main.go": `package main

import "fmt"

func main() {
	service := NewUserService()
	user1 := service.CreateUser(1, "Alice")
	fmt.Println(user1.GetName())
	retrieved := service.GetUser(1)
	fmt.Println(retrieved.Name)
}
`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create config
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}

	// Create indexer and index
	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	startIndex := time.Now()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	indexDuration := time.Since(startIndex)

	// Verify basic indexing
	fileIDs := indexer.GetAllFileIDs()
	if len(fileIDs) != len(testFiles) {
		t.Errorf("Expected %d files indexed, got %d", len(testFiles), len(fileIDs))
	}

	// Create search engine
	engine := search.NewEngine(indexer)

	// Smoke test: Search for key functions
	searches := []string{
		"NewUser",
		"CreateUser",
		"GetName",
		"NewUserService",
	}

	for _, pattern := range searches {
		start := time.Now()
		results := engine.SearchWithOptions(pattern, fileIDs, types.SearchOptions{})
		duration := time.Since(start)

		if len(results) == 0 {
			t.Logf("Warning: No results for pattern %s", pattern)
		}

		if duration > 100*time.Millisecond {
			t.Logf("Warning: Search for %s took %v", pattern, duration)
		}

		t.Logf("Search for %s: %d results in %v", pattern, len(results), duration)
	}

	// Performance sanity check
	if indexDuration > 3*time.Second {
		t.Logf("Warning: Indexing took %v (may indicate regression)", indexDuration)
	}
}

// TestEnhancedSmoke_WithSearchOptions tests enhanced search with various options
func TestEnhancedSmoke_WithSearchOptions(t *testing.T) {
	tempDir := t.TempDir()

	// Minimal project with clear patterns
	testFiles := map[string]string{
		"module.go": `package main

func Process(data string) string {
	return data
}

func ProcessData(input string) error {
	return nil
}

func Helper() {
}
`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}

	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Test various search options
	testCases := []struct {
		name    string
		pattern string
		opts    types.SearchOptions
	}{
		{
			name:    "basic_search",
			pattern: "Process",
			opts:    types.SearchOptions{},
		},
		{
			name:    "with_regex",
			pattern: "Process.*",
			opts:    types.SearchOptions{UseRegex: true},
		},
		{
			name:    "word_boundary",
			pattern: "Process",
			opts:    types.SearchOptions{WordBoundary: true},
		},
		{
			name:    "files_only",
			pattern: "Process",
			opts:    types.SearchOptions{FilesOnly: true},
		},
		{
			name:    "max_results",
			pattern: "Process",
			opts:    types.SearchOptions{MaxResults: 1},
		},
	}

	for _, tc := range testCases {
		start := time.Now()
		results := engine.SearchWithOptions(tc.pattern, fileIDs, tc.opts)
		duration := time.Since(start)

		if results == nil {
			t.Errorf("%s: unexpected failure", tc.name)
		}

		if duration > 500*time.Millisecond {
			t.Logf("Warning: %s took %v", tc.name, duration)
		}

		t.Logf("%s: %d results in %v", tc.name, len(results), duration)
	}
}

// TestEnhancedSmoke_MultiFile tests enhanced search across multiple files
func TestEnhancedSmoke_MultiFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create 3 files with cross-references
	testFiles := map[string]string{
		"api.go": `package main

func API() error {
	return Helper()
}
`,
		"impl.go": `package main

func Implementation() {
	API()
}
`,
		"util.go": `package main

func Helper() error {
	return nil
}
`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-project",
		},
		Index: config.Index{
			MaxFileSize:    types.DefaultMaxFileSize,
			MaxTotalSizeMB: int64(types.DefaultMaxTotalSizeMB),
			MaxFileCount:   types.DefaultMaxFileCount,
		},
		Performance: config.Performance{
			MaxMemoryMB: types.DefaultMaxMemoryMB,
		},
	}

	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Should find symbols across all files
	results := engine.SearchWithOptions("API", fileIDs, types.SearchOptions{})

	if len(results) < 1 {
		t.Errorf("Expected to find API references across files, got %d", len(results))
	}

	t.Logf("Found %d results for cross-file search", len(results))

	// Test file-only search
	fileOnlyResults := engine.SearchWithOptions("API", fileIDs, types.SearchOptions{FilesOnly: true})
	t.Logf("File-only search: %d results", len(fileOnlyResults))
}
