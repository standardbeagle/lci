package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/testhelpers"
)

// TestNewServer tests the new server.
func TestNewServer(t *testing.T) {
	cfg := testhelpers.NewTestConfigBuilder("/test/path").Build()

	// Create MasterIndex
	gi := indexing.NewMasterIndex(cfg)

	// Create MCP server
	server, err := NewServer(gi, cfg)
	require.NoError(t, err, "NewServer should not return error")
	require.NotNil(t, server, "NewServer should return non-nil server")

	// Verify server fields
	assert.NotNil(t, server.goroutineIndex, "Server should have goroutineIndex")
	assert.Equal(t, gi, server.goroutineIndex, "Server should use provided MasterIndex")
	assert.Equal(t, cfg, server.cfg, "Server should store config")
	assert.NotNil(t, server.server, "Server should create MCP server")
	assert.NotNil(t, server.diagnosticLogger, "Server should create logger")
}

// TestServerStructure tests the server structure.
func TestServerStructure(t *testing.T) {
	// Test that the Server struct has all required fields
	var s Server

	// These should compile without error, indicating the fields exist
	_ = s.goroutineIndex
	_ = s.cfg
	_ = s.server
	_ = s.diagnosticLogger

	// This verifies the struct has the expected shape
	assert.NotNil(t, &s, "Server struct should be properly defined")
}
