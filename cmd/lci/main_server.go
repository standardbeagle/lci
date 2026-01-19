package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/server"
	"github.com/urfave/cli/v2"
)

// serverCommand starts the persistent index server
func serverCommand(c *cli.Context) error {
	// Load configuration
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return err
	}

	// Create server
	srv, err := server.NewIndexServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Set project-specific socket path
	socketPath := server.GetSocketPathForRoot(cfg.Project.Root)
	srv.SetSocketPath(socketPath)

	// Start server
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	fmt.Printf("Index server started successfully\n")
	fmt.Printf("Socket: %s\n", socketPath)
	fmt.Printf("Root: %s\n", cfg.Project.Root)
	fmt.Printf("\nUse 'lci shutdown' to stop the server\n")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal or server shutdown
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
	case <-func() chan struct{} {
		ch := make(chan struct{})
		go func() {
			srv.Wait()
			close(ch)
		}()
		return ch
	}():
		fmt.Println("Server shutdown requested")
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	fmt.Println("Server shut down cleanly")
	return nil
}

// shutdownCommand sends a shutdown request to the running server
func shutdownCommand(c *cli.Context) error {
	// Load configuration to get project root
	cfg, err := loadConfigWithOverrides(c)
	if err != nil {
		return err
	}

	// Get project-specific socket path
	socketPath := server.GetSocketPathForRoot(cfg.Project.Root)
	client := server.NewClientWithSocket(socketPath)

	// Check if server is running
	if !client.IsServerRunning() {
		return fmt.Errorf("no server is running for root: %s", cfg.Project.Root)
	}

	force := c.Bool("force")

	fmt.Printf("Shutting down server for root: %s\n", cfg.Project.Root)
	if err := client.Shutdown(force); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	// Wait a moment to confirm shutdown
	time.Sleep(500 * time.Millisecond)

	if client.IsServerRunning() {
		return fmt.Errorf("server did not shut down")
	}

	fmt.Println("Server shut down successfully")
	return nil
}

// ensureServerRunning checks if the index server is running, and starts it if not
// It uses a project-specific socket path based on the configured root directory
func ensureServerRunning(cfg *config.Config) (*server.Client, error) {
	// Get project-specific socket path
	socketPath := server.GetSocketPathForRoot(cfg.Project.Root)
	client := server.NewClientWithSocket(socketPath)

	// Check if server is already running for this project
	if client.IsServerRunning() {
		return client, nil
	}

	// Server not running - start it in background
	fmt.Fprintln(os.Stderr, "Index server not running, starting in background...")

	// Get path to current executable
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start server as daemon with the correct root directory
	args := []string{"server"}
	if cfg.Project.Root != "" && cfg.Project.Root != "." {
		args = append([]string{"--root", cfg.Project.Root}, args...)
	}
	cmd := exec.Command(executable, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start the process detached
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	// Detach from the process so it continues after we exit
	if err := cmd.Process.Release(); err != nil {
		return nil, fmt.Errorf("failed to detach server process: %w", err)
	}

	// Wait for server to be ready (with timeout)
	fmt.Fprintln(os.Stderr, "Waiting for index server to be ready...")
	if err := client.WaitForReady(30 * time.Second); err != nil {
		return nil, fmt.Errorf("server did not become ready: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Index server ready")
	return client, nil
}

// startSharedIndexServer starts the index server in-process with a given MasterIndex
// This allows the MCP to share its index with CLI commands via RPC
func startSharedIndexServer(cfg *config.Config, indexer *indexing.MasterIndex) (*server.IndexServer, error) {
	// Get the search engine from the indexer
	// The MasterIndex should have a search engine set by the time this is called
	searchEngine := indexer.GetSearchEngine()

	// Create server with the existing index
	srv, err := server.NewIndexServerWithIndex(cfg, indexer, searchEngine)
	if err != nil {
		return nil, fmt.Errorf("failed to create index server: %w", err)
	}

	// Start the server (this creates the Unix socket and begins listening)
	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("failed to start index server: %w", err)
	}

	return srv, nil
}
