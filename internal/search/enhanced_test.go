package search_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	testhelpers "github.com/standardbeagle/lci/internal/testing"
)

// TestEnhancedSymbolReferenceCount tests that enhanced symbols have proper reference counting
func TestEnhancedSymbolReferenceCount(t *testing.T) {
	// Create a known good Go project with clear references
	tempDir := t.TempDir()

	// Write test files with known reference relationships
	testFiles := map[string]string{
		"types.go": `package main

// User represents a user in the system
type User struct {
	ID   int
	Name string
}

// NewUser creates a new user
func NewUser(id int, name string) *User {
	return &User{ID: id, Name: name}
}

// GetName returns the user's name
func (u *User) GetName() string {
	return u.Name
}
`,
		"service.go": `package main

// UserService handles user operations
type UserService struct {
	users map[int]*User
}

// NewUserService creates a new user service
func NewUserService() *UserService {
	return &UserService{
		users: make(map[int]*User),
	}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(id int, name string) *User {
	user := NewUser(id, name) // Reference to NewUser function
	s.users[id] = user
	return user
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id int) *User {
	return s.users[id]
}
`,
		"main.go": `package main

import "fmt"

func main() {
	service := NewUserService() // Reference to NewUserService
	
	// Create some users
	user1 := service.CreateUser(1, "Alice") // Reference to CreateUser
	user2 := service.CreateUser(2, "Bob")   // Reference to CreateUser
	
	// Print user names
	fmt.Println(user1.GetName()) // Reference to GetName
	fmt.Println(user2.GetName()) // Reference to GetName
	
	// Retrieve user
	retrieved := service.GetUser(1) // Reference to GetUser
	fmt.Println(retrieved.Name)
}
`,
		"user_test.go": `package main

import "testing"

// TestNewUser tests the new user.
func TestNewUser(t *testing.T) {
	user := NewUser(1, "Test") // Reference to NewUser
	if user.ID != 1 {
		t.Error("Wrong ID")
	}
	if user.GetName() != "Test" { // Reference to GetName
		t.Error("Wrong name")
	}
}

// TestUserService tests the user service.
func TestUserService(t *testing.T) {
	service := NewUserService() // Reference to NewUserService
	user := service.CreateUser(1, "Test") // Reference to CreateUser
	
	retrieved := service.GetUser(1) // Reference to GetUser
	if retrieved != user {
		t.Error("User not found")
	}
}
`,
	}

	// Write all test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Create indexer with configuration
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{
			Root: tempDir,
			Name: "test-references",
		},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			MaxTotalSizeMB:   100,
			MaxFileCount:     1000,
			FollowSymlinks:   false,
			RespectGitignore: false,
		},
		Performance: config.Performance{
			MaxMemoryMB:   100,
			MaxGoroutines: 4,
		},
		Search: config.Search{
			MaxResults:      100,
			MaxContextLines: 5,
		},
		Include: []string{"**/*.go"},
		Exclude: []string{},
	}

	// Create and populate the index
	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Verify indexing worked
	stats := indexer.GetIndexStats()
	t.Logf("Indexed %d files with %d symbols", stats.FileCount, stats.SymbolCount)
	if stats.FileCount != 4 {
		t.Errorf("Expected 4 files, got %d", stats.FileCount)
	}
	if stats.SymbolCount == 0 {
		t.Fatal("No symbols were indexed")
	}

	// Create search engine
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Define expected reference counts based on our test code
	// Note: The reference tracker may find more references than we manually count
	expectations := []testhelpers.ReferenceExpectation{
		{SymbolName: "User", MinIncoming: 0, MinOutgoing: 0, ExpectDeclaration: false}, // Type declaration, might not be found as symbol
		{SymbolName: "NewUser", MinIncoming: 2, MinOutgoing: 1, ExpectDeclaration: true},
		{SymbolName: "GetName", MinIncoming: 3, MinOutgoing: 0, ExpectDeclaration: true},
		{SymbolName: "UserService", MinIncoming: 0, MinOutgoing: 0, ExpectDeclaration: false}, // Type declaration
		{SymbolName: "NewUserService", MinIncoming: 2, MinOutgoing: 1, ExpectDeclaration: true},
		{SymbolName: "CreateUser", MinIncoming: 3, MinOutgoing: 1, ExpectDeclaration: true},
		{SymbolName: "GetUser", MinIncoming: 2, MinOutgoing: 0, ExpectDeclaration: true},
	}

	// Use shared validation
	testhelpers.ValidateEnhancedReferences(t, engine, fileIDs, expectations)

	// Test that basic search also shows reference counts
	t.Run("BasicSearchReferenceDisplay", func(t *testing.T) {
		testhelpers.ValidateSearchDisplaysReferences(t, engine, fileIDs, "NewUser")
	})
}

// TestEnhancedSearchPerformance tests that enhanced search meets performance requirements
func TestEnhancedSearchPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Use the same test setup as above
	tempDir := t.TempDir()

	// Create a larger codebase for performance testing
	for i := 0; i < 20; i++ {
		content := `package pkg%d

type Type%d struct {
	field1 string
	field2 int
}

func (t *Type%d) Method1() string {
	return t.field1
}

func (t *Type%d) Method2() int {
	return t.field2
}

func NewType%d() *Type%d {
	return &Type%d{}
}
`
		formatted := ""
		for j := 0; j < 10; j++ {
			formatted += content
		}

		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(filename, []byte(formatted), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Index and search
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			RespectGitignore: false,
		},
		Include: []string{"**/*.go"},
	}

	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index: %v", err)
	}

	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Time the detailed search
	start := time.Now()
	results := engine.SearchDetailed("Type", fileIDs, 50)
	duration := time.Since(start)

	t.Logf("Detailed search took %v for %d results", duration, len(results))

	// Should complete within reasonable time (under 100ms for this size)
	if duration > 100*time.Millisecond {
		t.Errorf("Detailed search too slow: %v", duration)
	}

	// Verify all results have proper enhanced data
	for i, result := range results {
		if result.RelationalData == nil {
			t.Errorf("Result %d missing relational data", i)
			continue
		}
		if result.RelationalData.Symbol.ID == 0 {
			t.Errorf("Result %d has invalid symbol ID", i)
		}
	}
}
