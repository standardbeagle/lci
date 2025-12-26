package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
)

// TestJavaScriptReferenceCount tests reference counting for JavaScript code
func TestJavaScriptReferenceCount(t *testing.T) {
	tempDir := t.TempDir()

	// JavaScript project with clear references
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

	// Create indexer
	cfg := &config.Config{
		Version: 1,
		Project: config.Project{Root: tempDir},
		Index: config.Index{
			MaxFileSize:      10 * 1024 * 1024,
			RespectGitignore: false,
		},
		Include: []string{"**/*.js"},
		Exclude: []string{},
	}

	indexer := indexing.NewMasterIndex(cfg)
	ctx := context.Background()
	if err := indexer.IndexDirectory(ctx, tempDir); err != nil {
		t.Fatalf("Failed to index directory: %v", err)
	}

	// Get search engine
	engine := NewEngine(indexer)
	fileIDs := indexer.GetAllFileIDs()

	// Test symbols with expected references
	expectedRefs := map[string]struct {
		minIncoming int
	}{
		"User":        {minIncoming: 1}, // Referenced by createUser
		"createUser":  {minIncoming: 3}, // Called 3 times
		"UserService": {minIncoming: 1}, // Instantiated once
		"getName":     {minIncoming: 2}, // Called 2 times
		"addUser":     {minIncoming: 1}, // Called once
		"getUser":     {minIncoming: 1}, // Called once
	}

	for symbol, expected := range expectedRefs {
		t.Run("JS_"+symbol, func(t *testing.T) {
			results := engine.SearchDetailed(symbol, fileIDs, 10)

			if len(results) == 0 {
				t.Fatalf("No results for %s", symbol)
			}

			// Check first result (likely declaration)
			result := results[0]
			if result.RelationalData == nil {
				t.Fatal("No relational data")
			}

			refCount := result.RelationalData.Symbol.RefStats.Total.IncomingCount
			t.Logf("%s: %d incoming references", symbol, refCount)

			if refCount < expected.minIncoming {
				t.Errorf("Expected at least %d incoming refs, got %d",
					expected.minIncoming, refCount)
			}
		})
	}
}
