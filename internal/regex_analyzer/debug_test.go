package regex_analyzer

import (
	"github.com/standardbeagle/lci/internal/types"
	"testing"
)

func TestDebugRegexIssue(t *testing.T) {
	engine := NewHybridRegexEngine(100, 100, nil)

	content1 := []byte("type UserService struct { db Database }")
	content2 := []byte("func NewUserService(db Database) *UserService { return &UserService{db: db} }")

	contentProvider := func(id types.FileID) ([]byte, bool) {
		switch id {
		case 1:
			return content1, true
		case 2:
			return content2, true
		}
		return nil, false
	}

	candidates := []types.FileID{1, 2}

	// Test literal pattern
	matches1, result1 := engine.SearchWithRegex("UserService", false, contentProvider, candidates)
	t.Logf("Literal 'UserService': %d matches, path=%d", len(matches1), result1.ExecutionPath)
	if len(matches1) == 0 {
		t.Error("Expected matches for literal 'UserService'")
	}

	// Test regex pattern
	matches2, result2 := engine.SearchWithRegex("User.*Service", false, contentProvider, candidates)
	t.Logf("Regex 'User.*Service': %d matches, path=%d", len(matches2), result2.ExecutionPath)
	if len(matches2) == 0 {
		t.Error("Expected matches for regex 'User.*Service'")
	}
}
