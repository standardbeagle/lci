package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// TestParserIntegration tests parser functionality across multiple languages
func TestParserIntegration(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		code     string
		expected []expectedSymbol
	}{
		{
			name:     "go_basic_functions",
			filename: "test.go",
			code: `package main

import "fmt"

// Calculate adds two numbers
func Calculate(a, b int) int {
	return a + b
}

type Calculator struct {
	precision int
}

func (c *Calculator) Add(a, b float64) float64 {
	return a + b
}

func main() {
	fmt.Println(Calculate(1, 2))
}
`,
			expected: []expectedSymbol{
				{name: "Calculate", symbolType: types.SymbolTypeFunction},
				{name: "Calculator", symbolType: types.SymbolTypeStruct},
				{name: "Add", symbolType: types.SymbolTypeMethod},
				{name: "main", symbolType: types.SymbolTypeFunction},
			},
		},
		{
			name:     "javascript_classes_and_functions",
			filename: "test.js",
			code: `// JavaScript test file
class UserManager {
	constructor(database) {
		this.db = database;
		this.cache = new Map();
	}
	
	async getUser(id) {
		if (this.cache.has(id)) {
			return this.cache.get(id);
		}
		
		const user = await fetchData('/users/' + id);
		this.cache.set(id, user);
		return user;
	}
	
	static validateUser(user) {
		return user && user.id && user.name;
	}
}

function fetchData(url) {
	return fetch(url).then(r => r.json());
}

const processUser = async (id) => {
	const manager = new UserManager();
	return await manager.getUser(id);
};
`,
			expected: []expectedSymbol{
				{name: "UserManager", symbolType: types.SymbolTypeClass},
				{name: "constructor", symbolType: types.SymbolTypeMethod},
				{name: "getUser", symbolType: types.SymbolTypeMethod},
				{name: "validateUser", symbolType: types.SymbolTypeMethod},
				{name: "fetchData", symbolType: types.SymbolTypeFunction},
				{name: "processUser", symbolType: types.SymbolTypeVariable},
			},
		},
		{
			name:     "python_comprehensive",
			filename: "test.py",
			code: `#!/usr/bin/env python3
"""Module for user management."""

from typing import Optional, Dict, List
import asyncio

class UserDatabase:
    """Handles user database operations."""
    
    def __init__(self, connection_string: str):
        self.connection = connection_string
        self._cache: Dict[int, dict] = {}
    
    async def get_user(self, user_id: int) -> Optional[dict]:
        """Fetch user by ID."""
        if user_id in self._cache:
            return self._cache[user_id]
        
        # Simulate database fetch
        user = await self._fetch_from_db(user_id)
        if user:
            self._cache[user_id] = user
        return user
    
    async def _fetch_from_db(self, user_id: int) -> Optional[dict]:
        """Internal method to fetch from database."""
        await asyncio.sleep(0.1)
        return {"id": user_id, "name": f"User{user_id}"}

def validate_user(user: dict) -> bool:
    """Validate user data."""
    return bool(user.get("id") and user.get("name"))

async def main():
    """Main entry point."""
    db = UserDatabase("postgresql://localhost/users")
    user = await db.get_user(123)
    if validate_user(user):
        print(f"Valid user: {user}")

if __name__ == "__main__":
    asyncio.run(main())
`,
			expected: []expectedSymbol{
				{name: "UserDatabase", symbolType: types.SymbolTypeClass},
				{name: "__init__", symbolType: types.SymbolTypeMethod},
				{name: "get_user", symbolType: types.SymbolTypeMethod},
				{name: "_fetch_from_db", symbolType: types.SymbolTypeMethod},
				{name: "validate_user", symbolType: types.SymbolTypeFunction},
				{name: "main", symbolType: types.SymbolTypeFunction},
			},
		},
		{
			name:     "typescript_interfaces_and_types",
			filename: "test.ts",
			code: `// TypeScript advanced features
interface User {
	id: number;
	name: string;
	email?: string;
}

type UserRole = 'admin' | 'user' | 'guest';

interface AuthService {
	login(username: string, password: string): Promise<User>;
	logout(): void;
	getCurrentUser(): User | null;
}

class AuthServiceImpl implements AuthService {
	private currentUser: User | null = null;
	
	async login(username: string, password: string): Promise<User> {
		// Simulate authentication
		const user: User = {
			id: 1,
			name: username,
			email: ` + "`${username}@example.com`" + `
		};
		this.currentUser = user;
		return user;
	}
	
	logout(): void {
		this.currentUser = null;
	}
	
	getCurrentUser(): User | null {
		return this.currentUser;
	}
}

export function createAuthService(): AuthService {
	return new AuthServiceImpl();
}

export { User, UserRole, AuthService };
`,
			expected: []expectedSymbol{
				{name: "User", symbolType: types.SymbolTypeInterface},
				{name: "UserRole", symbolType: types.SymbolTypeType},
				{name: "AuthService", symbolType: types.SymbolTypeInterface},
				{name: "AuthServiceImpl", symbolType: types.SymbolTypeClass},
				{name: "login", symbolType: types.SymbolTypeMethod},
				{name: "logout", symbolType: types.SymbolTypeMethod},
				{name: "getCurrentUser", symbolType: types.SymbolTypeMethod},
				{name: "createAuthService", symbolType: types.SymbolTypeFunction},
			},
		},
		{
			name:     "rust_traits_and_impls",
			filename: "test.rs",
			code: `//! Rust module for data processing

use std::collections::HashMap;

/// Trait for data processors
pub trait DataProcessor {
    fn process(&self, data: &str) -> Result<String, ProcessError>;
    fn validate(&self, data: &str) -> bool {
        !data.is_empty()
    }
}

#[derive(Debug)]
pub struct ProcessError {
    message: String,
}

/// JSON processor implementation
pub struct JsonProcessor {
    strict_mode: bool,
}

impl JsonProcessor {
    pub fn new(strict: bool) -> Self {
        JsonProcessor { strict_mode: strict }
    }
}

impl DataProcessor for JsonProcessor {
    fn process(&self, data: &str) -> Result<String, ProcessError> {
        if !self.validate(data) {
            return Err(ProcessError {
                message: "Empty data".to_string(),
            });
        }
        Ok(format!("Processed: {}", data))
    }
}

pub mod utils {
    pub fn sanitize_input(input: &str) -> String {
        input.trim().to_string()
    }
}
`,
			expected: []expectedSymbol{
				{name: "DataProcessor", symbolType: types.SymbolTypeTrait},
				{name: "process", symbolType: types.SymbolTypeMethod},
				{name: "validate", symbolType: types.SymbolTypeMethod},
				{name: "ProcessError", symbolType: types.SymbolTypeStruct},
				{name: "JsonProcessor", symbolType: types.SymbolTypeStruct},
				{name: "new", symbolType: types.SymbolTypeMethod},
				{name: "utils", symbolType: types.SymbolTypeModule},
				{name: "sanitize_input", symbolType: types.SymbolTypeFunction},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTreeSitterParser()

			// Parse the code
			boundaries, symbols, _ := parser.ParseFile(tt.filename, []byte(tt.code))

			// Verify we got boundaries
			if len(boundaries) == 0 && len(symbols) > 0 {
				t.Error("Expected boundaries for symbols")
			}

			// Verify all expected symbols were found
			for _, expected := range tt.expected {
				found := false
				for _, symbol := range symbols {
					if symbol.Name == expected.name && symbol.Type == expected.symbolType {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("Expected symbol not found: %s (%v)", expected.name, expected.symbolType)
					t.Logf("Available symbols:")
					for _, s := range symbols {
						t.Logf("  - %s (%v)", s.Name, s.Type)
					}
				}
			}
		})
	}
}

// TestParserEdgeCases_VerifiesHandlingOfComplexAndInvalidSyntax ensures parser handles tricky constructs gracefully
// Renamed from TestEdgeCases to avoid duplicate with search edge case test
func TestParserEdgeCases_VerifiesHandlingOfComplexAndInvalidSyntax(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		filename string
		code     string
		check    func(t *testing.T, boundaries []types.BlockBoundary, symbols []types.Symbol, imports []types.Import)
	}{
		{
			name:     "empty_file",
			filename: "empty.go",
			code:     "",
			check: func(t *testing.T, boundaries []types.BlockBoundary, symbols []types.Symbol, imports []types.Import) {
				if len(symbols) != 0 {
					t.Errorf("Empty file should have no symbols, got %d", len(symbols))
				}
			},
		},
		{
			name:     "syntax_error",
			filename: "error.go",
			code: `package main

func broken( {
	// Missing closing paren
}`,
			check: func(t *testing.T, boundaries []types.BlockBoundary, symbols []types.Symbol, imports []types.Import) {
				// Should still extract what it can
				hasFunc := false
				for _, s := range symbols {
					if s.Name == "broken" {
						hasFunc = true
						break
					}
				}
				if !hasFunc {
					t.Error("Should extract function even with syntax error")
				}
			},
		},
		{
			name:     "unicode_identifiers",
			filename: "unicode.go",
			code: `package main

func 世界() string {
	return "Hello, World!"
}

type 数据 struct {
	值 int
}`,
			check: func(t *testing.T, boundaries []types.BlockBoundary, symbols []types.Symbol, imports []types.Import) {
				expectedNames := []string{"世界", "数据"}
				for _, expected := range expectedNames {
					found := false
					for _, s := range symbols {
						if s.Name == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Unicode symbol %s not found", expected)
					}
				}
			},
		},
		{
			name:     "nested_functions",
			filename: "nested.js",
			code: `function outer() {
	function inner() {
		function deeplyNested() {
			return 42;
		}
		return deeplyNested();
	}
	return inner();
}`,
			check: func(t *testing.T, boundaries []types.BlockBoundary, symbols []types.Symbol, imports []types.Import) {
				expectedFuncs := []string{"outer", "inner", "deeplyNested"}
				for _, expected := range expectedFuncs {
					found := false
					for _, s := range symbols {
						if s.Name == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Nested function %s not found", expected)
					}
				}
			},
		},
		{
			name:     "complex_imports",
			filename: "imports.js",
			code: `import React, { useState, useEffect } from 'react';
import * as utils from './utils';
import { 
	helper1,
	helper2 as h2,
	helper3
} from '../helpers';
import defaultExport from 'module';`,
			check: func(t *testing.T, boundaries []types.BlockBoundary, symbols []types.Symbol, imports []types.Import) {
				if len(imports) < 4 {
					t.Errorf("Expected at least 4 imports, got %d", len(imports))
				}

				// Check for specific import paths
				paths := make(map[string]bool)
				for _, imp := range imports {
					paths[imp.Path] = true
				}

				// Note: Import.Path contains the import path from the file
				// For now just verify we have imports
				if len(paths) == 0 {
					t.Error("No import paths found")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boundaries, symbols, imports := parser.ParseFile(tt.filename, []byte(tt.code))
			tt.check(t, boundaries, symbols, imports)
		})
	}
}

// TestPerformance tests parser performance
// Run sequentially to avoid timeout under parallel load
func TestPerformance(t *testing.T) {
	parser := NewTreeSitterParser()

	// Generate a large file
	var sb strings.Builder
	sb.WriteString("package main\n\n")

	// Use smaller workload in short mode (100 functions) or when running concurrent tests
	// Full workload (1000 functions) only for dedicated performance testing
	numFunctions := 1000
	expectedTime := 8000 * time.Millisecond // 8s: ~2s standalone + 6s test suite overhead

	if testing.Short() {
		numFunctions = 100                     // 10x smaller for quick validation
		expectedTime = 1000 * time.Millisecond // Proportionally lower timeout
	}

	for i := 0; i < numFunctions; i++ {
		fmt.Fprintf(&sb, "func Function%d() int {\n\treturn %d\n}\n\n", i, i)
	}

	largeFile := sb.String()

	// Time the parsing
	start := time.Now()
	boundaries, symbols, _ := parser.ParseFile("large.go", []byte(largeFile))
	elapsed := time.Since(start)

	// Verify results
	if len(symbols) < numFunctions {
		t.Errorf("Expected at least %d symbols, got %d", numFunctions, len(symbols))
	}

	if len(boundaries) < numFunctions {
		t.Errorf("Expected at least %d boundaries, got %d", numFunctions, len(boundaries))
	}

	// Performance check - validated with CPU/memory profiling
	//
	// === PROFILER SUMMARY (Date: 2025-10-23) ===
	// Test: 1000 Go functions, ~100KB generated code
	// Baseline: 2.2-2.4s isolated, 2.5-3.5s in CI
	//
	// CPU Profile (2.27s total, 98.28% utilization):
	//   Top hotspots (cumulative):
	//     - runtime.cgocall:           1.64s (72.25%) - CGO overhead for C library calls
	//     - Node.Child():              1.83s (80.62%) - Tree-sitter AST traversal
	//     - detectContextAttributes:   1.61s (70.93%) - Context detection across all symbols
	//     - extractRelationalData:     0.83s (36.56%) - Reference and scope extraction
	//     - extractBasicSymbols:       0.80s (35.24%) - Symbol extraction from AST
	//     - buildEnhancedSymbols:      0.54s (23.79%) - Enhanced symbol construction
	//
	//   Bottleneck: Tree-sitter C library via CGO (72% of total time)
	//   Note: Defensive buffer copy is <1%, not visible in profile
	//
	// Memory Profile (189.69MB total allocations):
	//   Top allocations:
	//     - tree-sitter Node objects:  65.50MB (34.53%) - AST node allocations
	//     - strings.Split:             63.83MB (33.65%) - String splitting operations
	//     - extractDocComment:         46.26MB (24.39%) - Documentation extraction
	//     - buildEnhancedSymbols:       6.66MB (3.51%) - Enhanced symbol data
	//     - extractBasicSymbols:        0.51MB (0.27%) - Basic symbol structures
	//
	//   Note: Defensive parserBuffer copy (100KB) not visible in allocations (<0.1%)
	//
	// Architecture implications:
	//   1. CGO is inherent cost of tree-sitter - cannot be eliminated
	//   2. String operations dominate memory (64MB) - potential optimization target
	//   3. Doc extraction is expensive (46MB) - consider lazy evaluation
	//   4. Defensive copy validated as negligible overhead
	//
	// Threshold rationale:
	//   - Baseline: 2.3s (profiled)
	//   - CI overhead: +0.5-1.0s (environment variability)
	//   - Safety margin: +1.2s (resource contention tolerance)
	//   - Total: 5.0s threshold
	//
	// Note: Test failed at 4.38s in CI (2025-10-23) despite 2.76s in isolation,
	//       confirming need for adequate CI headroom.
	//
	// To reproduce profiling:
	//   go test -run TestPerformance -cpuprofile=cpu.prof -memprofile=mem.prof ./internal/parser
	//   go tool pprof -top -cum cpu.prof
	//   go tool pprof -alloc_space -top mem.prof
	if elapsed > expectedTime {
		t.Errorf("Parsing took too long: %v (expected < %v for tree-sitter parsing of %d symbols)\n"+
			"Profile with: go test -run TestPerformance -cpuprofile=cpu.prof -memprofile=mem.prof ./internal/parser\n"+
			"Analyze: go tool pprof -top -cum cpu.prof",
			elapsed, expectedTime, numFunctions)
	}

	t.Logf("Parsed %d symbols in %v", len(symbols), elapsed)
}

// TestEnhancedParsing tests enhanced parsing with references
func TestEnhancedParsing(t *testing.T) {
	parser := NewTreeSitterParser()

	code := `package main

type User struct {
	ID   int
	Name string
}

func GetUser(id int) *User {
	return &User{ID: id, Name: "Test"}
}

func ProcessUser(u *User) {
	if u.ID > 0 {
		println(u.Name)
	}
}

func main() {
	user := GetUser(123)
	ProcessUser(user)
}`

	// Test enhanced parsing
	boundaries, symbols, imports, enhanced, references, scopes := parser.ParseFileEnhanced("test.go", []byte(code))

	// Basic checks
	if len(symbols) == 0 {
		t.Fatal("No symbols extracted")
	}

	if len(enhanced) == 0 {
		t.Fatal("No enhanced symbols extracted")
	}

	// Check references exist
	if len(references) == 0 {
		t.Error("Expected references to be extracted")
	}

	// Check scope information
	if len(scopes) == 0 {
		t.Error("Expected scope information")
	}

	// Verify enhanced symbols have additional data
	for _, es := range enhanced {
		if es.Name == "User" {
			// Check that enhanced symbol has reference information
			if es.RefStats.Total.IncomingCount == 0 && es.RefStats.Total.OutgoingCount == 0 {
				t.Log("User struct has no references tracked")
			}
		}
	}

	_ = boundaries
	_ = imports
}

// BenchmarkParsing benchmarks parser performance
func BenchmarkParsing(b *testing.B) {
	parser := NewTreeSitterParser()

	code := `package main

import (
	"fmt"
	"strings"
)

type Service struct {
	name string
	port int
}

func (s *Service) Start() error {
	fmt.Printf("Starting %s on port %d\n", s.name, s.port)
	return nil
}

func (s *Service) Stop() error {
	fmt.Printf("Stopping %s\n", s.name)
	return nil
}

func main() {
	svc := &Service{name: "api", port: 8080}
	if err := svc.Start(); err != nil {
		panic(err)
	}
	defer svc.Stop()
}`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		parser.ParseFile("bench.go", []byte(code))
	}
}

// BenchmarkEnhancedParsing benchmarks enhanced parsing
func BenchmarkEnhancedParsing(b *testing.B) {
	parser := NewTreeSitterParser()

	// Use the same code as above
	code := `package main

import (
	"fmt"
	"strings"
)

type Service struct {
	name string
	port int
}

func (s *Service) Start() error {
	fmt.Printf("Starting %s on port %d\n", s.name, s.port)
	return nil
}

func (s *Service) Stop() error {
	fmt.Printf("Stopping %s\n", s.name)
	return nil
}

func main() {
	svc := &Service{name: "api", port: 8080}
	if err := svc.Start(); err != nil {
		panic(err)
	}
	defer svc.Stop()
}`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		parser.ParseFileEnhanced("bench.go", []byte(code))
	}
}

// Helper types
type expectedSymbol struct {
	name       string
	symbolType types.SymbolType
}
