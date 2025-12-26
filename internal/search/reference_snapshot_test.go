package search_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/search"
	testhelpers "github.com/standardbeagle/lci/internal/testing"
)

// TestGoReferenceSnapshot tests Go reference counting using snapshots
func TestGoReferenceSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	// Create test project files
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

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Create and run indexer
	cfg := createSnapshotTestConfig(tempDir, "**/*.go")
	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Get stats and create engine
	stats := indexer.GetIndexStats()
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Build snapshot from actual results
	snapshot := &testhelpers.ProjectSnapshot{
		ProjectName: "go-user-service",
		Language:    "go",
		FileCount:   stats.FileCount,
		SymbolCount: stats.SymbolCount,
		References:  make(map[string]testhelpers.ReferenceSnapshot),
	}

	// Symbols to check (functions only - types not included in snapshot)
	symbols := []string{"NewUser", "GetName", "NewUserService", "CreateUser", "GetUser"}

	for _, symbolName := range symbols {
		results := engine.SearchDetailed(symbolName, fileIDs, 10)
		if len(results) > 0 {
			// Find declaration
			for _, result := range results {
				if result.RelationalData != nil && result.RelationalData.Symbol.Name == symbolName {
					sym := &result.RelationalData.Symbol
					snapshot.References[symbolName] = testhelpers.ReferenceSnapshot{
						SymbolName:    symbolName,
						IncomingCount: sym.RefStats.Total.IncomingCount,
						OutgoingCount: sym.RefStats.Total.OutgoingCount,
						HasID:         sym.ID != 0,
						HasFileID:     sym.FileID != 0,
						IncomingRefs:  len(sym.IncomingRefs),
						OutgoingRefs:  len(sym.OutgoingRefs),
					}
					break
				}
			}
		}
	}

	// Assert snapshot
	testhelpers.AssertSnapshot(t, "go-reference-counting", snapshot)
}

// TestJavaScriptReferenceSnapshot tests JavaScript reference counting using snapshots
func TestJavaScriptReferenceSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	// JavaScript test files
	testFiles := map[string]string{
		"user.js": `// User class
export class User {
  constructor(id, name) {
    this.id = id;
    this.name = name;
  }
  
  getName() {
    return this.name;
  }
}

export function createUser(id, name) {
  return new User(id, name);
}
`,
		"service.js": `import { User, createUser } from './user.js';

export class UserService {
  constructor() {
    this.users = new Map();
  }
  
  addUser(id, name) {
    const user = createUser(id, name); // Reference to createUser
    this.users.set(id, user);
    return user;
  }
  
  getUser(id) {
    return this.users.get(id);
  }
}
`,
		"main.js": `import { UserService } from './service.js';
import { createUser } from './user.js';

const service = new UserService(); // Reference to UserService

// Direct user creation
const user1 = createUser(1, 'Alice'); // Reference to createUser
console.log(user1.getName()); // Reference to getName

// Service-based user creation  
const user2 = service.addUser(2, 'Bob'); // Reference to addUser
console.log(user2.getName()); // Reference to getName

// Retrieve user
const retrieved = service.getUser(2); // Reference to getUser
console.log(retrieved.name);
`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Create and run indexer
	cfg := createSnapshotTestConfig(tempDir, "**/*.js")
	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Get stats and create engine
	stats := indexer.GetIndexStats()
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Build snapshot
	snapshot := &testhelpers.ProjectSnapshot{
		ProjectName: "js-user-service",
		Language:    "javascript",
		FileCount:   stats.FileCount,
		SymbolCount: stats.SymbolCount,
		References:  make(map[string]testhelpers.ReferenceSnapshot),
	}

	// Symbols to check
	symbols := []string{"User", "createUser", "getName", "UserService", "addUser", "getUser"}

	for _, symbolName := range symbols {
		results := engine.SearchDetailed(symbolName, fileIDs, 10)
		if len(results) > 0 {
			// Find declaration
			for _, result := range results {
				if result.RelationalData != nil && result.RelationalData.Symbol.Name == symbolName {
					sym := &result.RelationalData.Symbol
					snapshot.References[symbolName] = testhelpers.ReferenceSnapshot{
						SymbolName:    symbolName,
						IncomingCount: sym.RefStats.Total.IncomingCount,
						OutgoingCount: sym.RefStats.Total.OutgoingCount,
						HasID:         sym.ID != 0,
						HasFileID:     sym.FileID != 0,
						IncomingRefs:  len(sym.IncomingRefs),
						OutgoingRefs:  len(sym.OutgoingRefs),
					}
					break
				}
			}
		}
	}

	// Assert snapshot
	testhelpers.AssertSnapshot(t, "js-reference-counting", snapshot)
}

// TestPythonReferenceSnapshot tests Python reference counting using snapshots
func TestPythonReferenceSnapshot(t *testing.T) {
	tempDir := t.TempDir()

	// Python test files
	testFiles := map[string]string{
		"user.py": `"""User module for user management."""

class User:
    """Represents a user in the system."""
    
    def __init__(self, user_id, name):
        self.id = user_id
        self.name = name
    
    def get_name(self):
        """Returns the user's name."""
        return self.name


def create_user(user_id, name):
    """Factory function to create a new user."""
    return User(user_id, name)
`,
		"service.py": `"""User service module."""

from user import User, create_user


class UserService:
    """Service for managing users."""
    
    def __init__(self):
        self.users = {}
    
    def add_user(self, user_id, name):
        """Add a new user to the service."""
        user = create_user(user_id, name)  # Reference to create_user
        self.users[user_id] = user
        return user
    
    def get_user(self, user_id):
        """Retrieve a user by ID."""
        return self.users.get(user_id)
`,
		"main.py": `"""Main application entry point."""

from service import UserService
from user import create_user


def main():
    """Main function."""
    service = UserService()  # Reference to UserService
    
    # Direct user creation
    user1 = create_user(1, "Alice")  # Reference to create_user
    print(user1.get_name())  # Reference to get_name
    
    # Service-based user creation
    user2 = service.add_user(2, "Bob")  # Reference to add_user
    print(user2.get_name())  # Reference to get_name
    
    # Retrieve user
    retrieved = service.get_user(2)  # Reference to get_user
    if retrieved:
        print(retrieved.name)


if __name__ == "__main__":
    main()
`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Create and run indexer
	cfg := createSnapshotTestConfig(tempDir, "**/*.py")
	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Get stats and create engine
	stats := indexer.GetIndexStats()
	engine := search.NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Build snapshot
	snapshot := &testhelpers.ProjectSnapshot{
		ProjectName: "python-user-service",
		Language:    "python",
		FileCount:   stats.FileCount,
		SymbolCount: stats.SymbolCount,
		References:  make(map[string]testhelpers.ReferenceSnapshot),
	}

	// Symbols to check
	symbols := []string{"User", "create_user", "get_name", "UserService", "add_user", "get_user", "main"}

	for _, symbolName := range symbols {
		results := engine.SearchDetailed(symbolName, fileIDs, 10)
		if len(results) > 0 {
			// Find declaration
			for _, result := range results {
				if result.RelationalData != nil && result.RelationalData.Symbol.Name == symbolName {
					sym := &result.RelationalData.Symbol
					snapshot.References[symbolName] = testhelpers.ReferenceSnapshot{
						SymbolName:    symbolName,
						IncomingCount: sym.RefStats.Total.IncomingCount,
						OutgoingCount: sym.RefStats.Total.OutgoingCount,
						HasID:         sym.ID != 0,
						HasFileID:     sym.FileID != 0,
						IncomingRefs:  len(sym.IncomingRefs),
						OutgoingRefs:  len(sym.OutgoingRefs),
					}
					break
				}
			}
		}
	}

	// Assert snapshot
	testhelpers.AssertSnapshot(t, "python-reference-counting", snapshot)
}

// Helper function to create test configuration
func createSnapshotTestConfig(rootDir string, includePattern string) *config.Config {
	return &config.Config{
		Version: 1,
		Project: config.Project{
			Root: rootDir,
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
		Include: []string{includePattern},
		Exclude: []string{},
	}
}
