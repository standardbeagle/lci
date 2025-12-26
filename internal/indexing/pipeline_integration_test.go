package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
)

// TestIntegration_SymbolStats tests symbol indexing and statistics
func TestIntegration_SymbolStats(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main

import "fmt"

const MaxRetries = 3
var globalCounter int

type Config struct {
	Host string
	Port int
}

func (c *Config) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func main() {
	cfg := &Config{Host: "localhost", Port: 8080}
	fmt.Println(cfg.GetAddress())
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	ctx := context.Background()
	err = mi.IndexDirectory(ctx, tmpDir)
	require.NoError(t, err)

	// Get symbol statistics
	stats := mi.GetSymbolStats()
	typeDistribution := mi.GetTypeDistribution()

	t.Logf("Total symbols: %d", stats.TotalSymbols)
	t.Logf("Type distribution: %+v", typeDistribution)

	// Verify we found expected symbol types
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeFunction], 1, "Should find functions")
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeMethod], 1, "Should find methods")
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeStruct], 1, "Should find structs")
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeVariable], 1, "Should find variables")
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeConstant], 1, "Should find constants")

	assert.Greater(t, stats.TotalSymbols, 5, "Should have multiple symbols")
	assert.Greater(t, stats.TotalFunctions, 0, "Should have functions")
}

// TestIntegration_MultipleFiles tests indexing across multiple files
func TestIntegration_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "types.go"), []byte(`package test

type User struct {
	ID       int
	Username string
}

type UserRepository interface {
	FindByID(id int) (*User, error)
	Save(user *User) error
}
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "repository.go"), []byte(`package test

type DBUserRepository struct {
	db interface{}
}

func (r *DBUserRepository) FindByID(id int) (*User, error) {
	return &User{ID: id}, nil
}

func (r *DBUserRepository) Save(user *User) error {
	return nil
}
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(`package test

type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUser(id int) (*User, error) {
	return s.repo.FindByID(id)
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	ctx := context.Background()
	err = mi.IndexDirectory(ctx, tmpDir)
	require.NoError(t, err)

	stats := mi.GetSymbolStats()
	refStats := mi.GetReferenceStats()

	t.Logf("Multi-file stats: Symbols=%d, Functions=%d, Methods=%d, References=%d",
		stats.TotalSymbols, stats.TotalFunctions, stats.TotalMethods, refStats.TotalReferences)

	assert.Greater(t, stats.TotalSymbols, 10, "Should have many symbols across files")
	assert.GreaterOrEqual(t, stats.TotalFunctions, 1, "Should have functions")
	assert.GreaterOrEqual(t, stats.TotalMethods, 2, "Should have FindByID and Save methods")
	// Reference tracking may vary based on implementation
	t.Logf("Cross-file reference tracking: %d references", refStats.TotalReferences)
}

// TestIntegration_Generics tests Go generics support
func TestIntegration_Generics(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "generics.go"), []byte(`package test

type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(item T) {
	s.items = append(s.items, item)
}

func (s *Stack[T]) Pop() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item, true
}

func Map[T, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	ctx := context.Background()
	err = mi.IndexDirectory(ctx, tmpDir)
	require.NoError(t, err)

	typeDistribution := mi.GetTypeDistribution()

	t.Logf("Generic symbols - Functions: %d, Methods: %d, Structs: %d",
		typeDistribution[types.SymbolTypeFunction],
		typeDistribution[types.SymbolTypeMethod],
		typeDistribution[types.SymbolTypeStruct])

	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeStruct], 1, "Should index Stack[T]")
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeMethod], 2, "Should index Push and Pop")
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeFunction], 1, "Should index Map function")
}

// TestIntegration_Interfaces tests interface and implementation indexing
func TestIntegration_Interfaces(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "interfaces.go"), []byte(`package test

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type ReadWriter interface {
	Reader
	Writer
	Close() error
}

type FileHandle struct {
	path string
}

func (f *FileHandle) Read(p []byte) (int, error) {
	return 0, nil
}

func (f *FileHandle) Write(p []byte) (int, error) {
	return len(p), nil
}

func (f *FileHandle) Close() error {
	return nil
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	ctx := context.Background()
	err = mi.IndexDirectory(ctx, tmpDir)
	require.NoError(t, err)

	stats := mi.GetSymbolStats()
	typeDistribution := mi.GetTypeDistribution()

	t.Logf("Interfaces: %d, Methods: %d, Structs: %d",
		typeDistribution[types.SymbolTypeInterface],
		typeDistribution[types.SymbolTypeMethod],
		typeDistribution[types.SymbolTypeStruct])

	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeInterface], 3, "Should find Reader, Writer, ReadWriter")
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeMethod], 3, "Should find Read, Write, Close")
	assert.GreaterOrEqual(t, typeDistribution[types.SymbolTypeStruct], 1, "Should find FileHandle")
	assert.GreaterOrEqual(t, stats.TotalMethods, 3, "Stats should track methods")
}

// TestIntegration_EmptyFiles tests handling of empty and minimal files
func TestIntegration_EmptyFiles(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "empty.go"), []byte(`package test
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "comments.go"), []byte(`package test

// This file only has comments
// No actual code symbols
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	mi := NewMasterIndex(cfg)
	defer mi.Close()

	ctx := context.Background()
	err = mi.IndexDirectory(ctx, tmpDir)
	require.NoError(t, err, "Should handle empty files gracefully")

	stats := mi.GetSymbolStats()
	t.Logf("Empty file stats: Symbols=%d", stats.TotalSymbols)

	// Should index files even if they have no symbols
	// (TotalSymbols might be 0, but indexing should not error)
}
