package server

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/version"
)

func TestBuildID_PingIncludesBuildID(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	cfg := &config.Config{
		Project: config.Project{Root: testDir},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Shutdown(context.Background())

	client := NewClientWithSocket(socketPath)
	ping, err := client.Ping()
	require.NoError(t, err)

	assert.NotEmpty(t, ping.BuildID, "Ping response should include BuildID")
	assert.Equal(t, version.BuildID(), ping.BuildID,
		"Server BuildID should match current binary's BuildID")
}

func TestBuildID_Deterministic(t *testing.T) {
	id1 := version.BuildID()
	id2 := version.BuildID()
	assert.Equal(t, id1, id2, "BuildID should be deterministic across calls")
	assert.NotEmpty(t, id1, "BuildID should not be empty")
}

func TestBuildID_SocketPathUniqueness(t *testing.T) {
	path1 := GetSocketPathForRoot("/some/project/a")
	path2 := GetSocketPathForRoot("/some/project/b")
	path3 := GetSocketPathForRoot("/some/project/a")

	assert.NotEqual(t, path1, path2, "Different roots should produce different socket paths")
	assert.Equal(t, path1, path3, "Same root should produce the same socket path")
}

func TestBuildID_SingleIndexPerFolder(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	cfg := &config.Config{
		Project: config.Project{Root: testDir},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	srv1, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv1.SetSocketPath(socketPath)
	err = srv1.Start()
	require.NoError(t, err)

	client1 := NewClientWithSocket(socketPath)
	require.True(t, client1.IsServerRunning(), "First server should be reachable")

	// Start() on the same server instance should fail (already running)
	err = srv1.Start()
	assert.Error(t, err, "Second Start() on same instance should fail")

	// Clean up
	srv1.Shutdown(context.Background())
}

func TestBuildID_MismatchTriggersShutdown(t *testing.T) {
	testDir := t.TempDir()
	socketPath := getTestSocketPath(t)
	defer os.Remove(socketPath)

	cfg := &config.Config{
		Project: config.Project{Root: testDir},
		Include: []string{"*.go"},
		Exclude: []string{},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	srv, err := NewIndexServer(cfg)
	require.NoError(t, err)
	srv.SetSocketPath(socketPath)
	srv.BuildIDOverride = "old-build-abc123"
	err = srv.Start()
	require.NoError(t, err)

	// Simulate the server main loop: wait for shutdown signal, then cleanup
	go func() {
		srv.Wait()
		srv.Shutdown(context.Background())
	}()

	client := NewClientWithSocket(socketPath)
	require.True(t, client.IsServerRunning())

	// Ping should return the override build ID
	ping, err := client.Ping()
	require.NoError(t, err)
	assert.Equal(t, "old-build-abc123", ping.BuildID)
	assert.NotEqual(t, version.BuildID(), ping.BuildID,
		"Override should differ from current binary's BuildID")

	// Shutdown via client (simulating what ensureServerRunning does on mismatch)
	err = client.Shutdown(false)
	require.NoError(t, err)

	// Wait for server to fully shut down
	time.Sleep(time.Second)
	assert.False(t, client.IsServerRunning(), "Server should be stopped after shutdown")
}
