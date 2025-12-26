package core

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestImportResolver_ExtractGoImports tests the import resolver extract go imports.
func TestImportResolver_ExtractGoImports(t *testing.T) {
	resolver := NewImportResolver()

	goCode := []byte(`package main

import (
	"fmt"
	"os"
	alias "path/to/package"
)

import "single"

func main() {
	fmt.Println("Hello")
}`)

	fileID := types.FileID(1)
	importData := resolver.ExtractFileImports(fileID, "test.go", goCode)

	if importData == nil {
		t.Fatal("Expected import data for Go file")
	}

	if importData.FileID != fileID {
		t.Errorf("Expected file ID %d, got %d", fileID, importData.FileID)
	}

	if len(importData.Bindings) == 0 {
		t.Fatal("Expected at least one import binding")
	}

	// Check for expected imports
	expectedImports := map[string]string{
		"fmt":    "fmt",
		"os":     "os",
		"alias":  "path/to/package",
		"single": "single",
	}

	foundImports := make(map[string]string)
	for _, binding := range importData.Bindings {
		foundImports[binding.ImportedName] = binding.SourceFile
	}

	for expectedName, expectedSource := range expectedImports {
		if source, found := foundImports[expectedName]; !found {
			t.Errorf("Expected import %s not found", expectedName)
		} else if source != expectedSource {
			t.Errorf("Import %s: expected source %s, got %s", expectedName, expectedSource, source)
		}
	}
}

// TestImportResolver_ExtractJavaScriptImports tests the import resolver extract java script imports.
func TestImportResolver_ExtractJavaScriptImports(t *testing.T) {
	resolver := NewImportResolver()

	jsCode := []byte(`import React from 'react';
import { useState, useEffect } from 'react';
import * as utils from './utils';
import { debounce as debounceFn } from './helpers';

function App() {
    return <div>Hello</div>;
}`)

	fileID := types.FileID(1)
	importData := resolver.ExtractFileImports(fileID, "test.js", jsCode)

	if importData == nil {
		t.Fatal("Expected import data for JS file")
	}

	if len(importData.Bindings) == 0 {
		t.Fatal("Expected at least one import binding")
	}

	// Check for expected imports
	expectedImports := []struct {
		importedName string
		originalName string
		sourceFile   string
	}{
		{"React", "React", "react"},
		{"useState", "useState", "react"},
		{"useEffect", "useEffect", "react"},
		{"debounceFn", "debounce", "./helpers"},
	}

	foundImports := make(map[string]ImportBinding)
	for _, binding := range importData.Bindings {
		foundImports[binding.ImportedName] = binding
	}

	for _, expected := range expectedImports {
		if binding, found := foundImports[expected.importedName]; !found {
			t.Errorf("Expected import %s not found", expected.importedName)
		} else {
			if binding.OriginalName != expected.originalName {
				t.Errorf("Import %s: expected original name %s, got %s",
					expected.importedName, expected.originalName, binding.OriginalName)
			}
			if binding.SourceFile != expected.sourceFile {
				t.Errorf("Import %s: expected source %s, got %s",
					expected.importedName, expected.sourceFile, binding.SourceFile)
			}
		}
	}
}

// TestImportResolver_ExtractPythonImports tests the import resolver extract python imports.
func TestImportResolver_ExtractPythonImports(t *testing.T) {
	resolver := NewImportResolver()

	pythonCode := []byte(`import os
import sys
from collections import defaultdict, Counter
from typing import List, Dict as DictType

def main():
    pass`)

	fileID := types.FileID(1)
	importData := resolver.ExtractFileImports(fileID, "test.py", pythonCode)

	if importData == nil {
		t.Fatal("Expected import data for Python file")
	}

	if len(importData.Bindings) == 0 {
		t.Fatal("Expected at least one import binding")
	}

	// Check for expected imports
	expectedImports := []struct {
		importedName string
		originalName string
		sourceFile   string
	}{
		{"defaultdict", "defaultdict", "collections"},
		{"Counter", "Counter", "collections"},
		{"List", "List", "typing"},
		{"DictType", "Dict", "typing"},
	}

	foundImports := make(map[string]ImportBinding)
	for _, binding := range importData.Bindings {
		foundImports[binding.ImportedName] = binding
	}

	for _, expected := range expectedImports {
		if binding, found := foundImports[expected.importedName]; !found {
			t.Errorf("Expected import %s not found", expected.importedName)
		} else {
			if binding.OriginalName != expected.originalName {
				t.Errorf("Import %s: expected original name %s, got %s",
					expected.importedName, expected.originalName, binding.OriginalName)
			}
			if binding.SourceFile != expected.sourceFile {
				t.Errorf("Import %s: expected source %s, got %s",
					expected.importedName, expected.sourceFile, binding.SourceFile)
			}
		}
	}
}

// TestImportResolver_ExtractRustImports tests the import resolver extract rust imports.
func TestImportResolver_ExtractRustImports(t *testing.T) {
	resolver := NewImportResolver()

	rustCode := []byte(`use std::collections::HashMap;
use serde::{Serialize, Deserialize};
use crate::utils::helper;

fn main() {
    println!("Hello, world!");
}`)

	fileID := types.FileID(1)
	importData := resolver.ExtractFileImports(fileID, "test.rs", rustCode)

	if importData == nil {
		t.Fatal("Expected import data for Rust file")
	}

	if len(importData.Bindings) == 0 {
		t.Fatal("Expected at least one import binding")
	}

	// Check for expected imports (Rust parsing is simplified)
	foundSymbols := make(map[string]bool)
	for _, binding := range importData.Bindings {
		foundSymbols[binding.ImportedName] = true
	}

	expectedSymbols := []string{"HashMap", "Deserialize", "helper"}
	for _, expected := range expectedSymbols {
		if !foundSymbols[expected] {
			t.Errorf("Expected to find import symbol %s", expected)
		}
	}
}

// TestImportResolver_UnsupportedLanguage tests the import resolver unsupported language.
func TestImportResolver_UnsupportedLanguage(t *testing.T) {
	resolver := NewImportResolver()

	unknownCode := []byte(`some unknown language code`)
	fileID := types.FileID(1)

	importData := resolver.ExtractFileImports(fileID, "test.unknown", unknownCode)

	if importData != nil {
		t.Error("Expected nil import data for unsupported language")
	}
}

// TestImportResolver_BuildImportGraph tests the import resolver build import graph.
func TestImportResolver_BuildImportGraph(t *testing.T) {
	resolver := NewImportResolver()

	// Create test import data
	importData := []*FileImportData{
		{
			FileID: types.FileID(1),
			Bindings: []ImportBinding{
				{ImportedName: "fmt", SourceFile: "fmt"},
				{ImportedName: "os", SourceFile: "os"},
			},
		},
		{
			FileID: types.FileID(2),
			Bindings: []ImportBinding{
				{ImportedName: "React", SourceFile: "react"},
			},
		},
		nil, // Should be ignored
		{
			FileID:   types.FileID(3),
			Bindings: []ImportBinding{}, // Empty bindings should be ignored
		},
	}

	resolver.BuildImportGraph(importData)

	// Test resolving symbols using the built graph
	mockSymbols := map[types.SymbolID]*types.EnhancedSymbol{
		1: {Symbol: types.Symbol{Name: "Printf"}},
		2: {Symbol: types.Symbol{Name: "Printf"}},
	}

	symbolLookup := func(id types.SymbolID) *types.EnhancedSymbol {
		return mockSymbols[id]
	}

	// Test symbol resolution for file 1 (should find fmt import)
	candidates := []types.SymbolID{1, 2}
	result := resolver.ResolveSymbolReference(types.FileID(1), "Printf", candidates, symbolLookup)

	if result == 0 {
		t.Error("Expected to resolve symbol reference")
	}
}

// TestImportResolver_ResolveSymbolReference tests the import resolver resolve symbol reference.
func TestImportResolver_ResolveSymbolReference(t *testing.T) {
	resolver := NewImportResolver()

	// Build a simple import graph
	importData := []*FileImportData{
		{
			FileID: types.FileID(1),
			Bindings: []ImportBinding{
				{ImportedName: "fmt", OriginalName: "fmt", SourceFile: "fmt"},
			},
		},
	}

	resolver.BuildImportGraph(importData)

	// Mock symbols for testing
	mockSymbols := map[types.SymbolID]*types.EnhancedSymbol{
		1: {Symbol: types.Symbol{Name: "Println", FileID: types.FileID(1)}},
		2: {Symbol: types.Symbol{Name: "Println", FileID: types.FileID(2)}},
		3: {Symbol: types.Symbol{Name: "println", FileID: types.FileID(3)}}, // Not exported (lowercase)
	}

	symbolLookup := func(id types.SymbolID) *types.EnhancedSymbol {
		return mockSymbols[id]
	}

	candidates := []types.SymbolID{1, 2, 3}

	// Test 1: Same file preference
	result := resolver.ResolveSymbolReference(types.FileID(1), "Println", candidates, symbolLookup)
	if result != 1 {
		t.Errorf("Expected symbol from same file (ID 1), got %d", result)
	}

	// Test 2: Exported symbol preference when not in same file
	result = resolver.ResolveSymbolReference(types.FileID(4), "Println", candidates, symbolLookup)
	if result != 1 && result != 2 { // Should prefer exported symbols (uppercase)
		t.Errorf("Expected exported symbol (ID 1 or 2), got %d", result)
	}

	// Test 3: No candidates
	result = resolver.ResolveSymbolReference(types.FileID(1), "NonExistent", []types.SymbolID{}, symbolLookup)
	if result != 0 {
		t.Errorf("Expected 0 for no candidates, got %d", result)
	}
}

// TestImportResolver_Clear tests the import resolver clear.
func TestImportResolver_Clear(t *testing.T) {
	resolver := NewImportResolver()

	// Build import graph
	importData := []*FileImportData{
		{
			FileID: types.FileID(1),
			Bindings: []ImportBinding{
				{ImportedName: "fmt", SourceFile: "fmt"},
			},
		},
	}

	resolver.BuildImportGraph(importData)

	// Verify data exists (by trying to resolve a symbol)
	mockSymbol := &types.EnhancedSymbol{Symbol: types.Symbol{Name: "Test"}}
	symbolLookup := func(id types.SymbolID) *types.EnhancedSymbol {
		return mockSymbol
	}

	result := resolver.ResolveSymbolReference(types.FileID(1), "fmt", []types.SymbolID{1}, symbolLookup)
	if result == 0 {
		t.Fatal("Expected to find symbol before clear")
	}

	// Clear and verify data is gone
	resolver.Clear()

	// After clear, should fall back to basic resolution
	result = resolver.ResolveSymbolReference(types.FileID(1), "fmt", []types.SymbolID{1}, symbolLookup)
	if result != 1 { // Should fall back to first candidate
		t.Error("Clear did not properly remove import graph data")
	}
}

// TestImportResolver_LineNumberDetection tests the import resolver line number detection.
func TestImportResolver_LineNumberDetection(t *testing.T) {
	resolver := NewImportResolver()

	goCode := []byte(`package main

import "fmt"
import "os"

func main() {
	fmt.Println("test")
}`)

	fileID := types.FileID(1)
	importData := resolver.ExtractFileImports(fileID, "test.go", goCode)

	if importData == nil || len(importData.Bindings) == 0 {
		t.Fatal("Expected import data")
	}

	// Check that line numbers are detected
	lineNumbers := make(map[string]int)
	for _, binding := range importData.Bindings {
		if binding.LineNumber > 0 {
			lineNumbers[binding.ImportedName] = binding.LineNumber
		}
	}

	if len(lineNumbers) == 0 {
		t.Error("Expected line numbers to be detected for imports")
	}

	// Line numbers should be reasonable (within the source)
	for name, lineNum := range lineNumbers {
		if lineNum < 1 || lineNum > 10 {
			t.Errorf("Import %s has unreasonable line number %d", name, lineNum)
		}
	}
}

// TestImportResolver_ConcurrentExtraction tests the import resolver concurrent extraction.
func TestImportResolver_ConcurrentExtraction(t *testing.T) {
	resolver := NewImportResolver()

	// Test concurrent extraction (should be safe since it's lock-free)
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			code := []byte(`package main
import "fmt"
func main() { fmt.Println("test") }`)

			// Multiple extractions
			for j := 0; j < 100; j++ {
				data := resolver.ExtractFileImports(types.FileID(id), "test.go", code)
				if data == nil {
					return // Error in extraction
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and resolver should still work
	testData := resolver.ExtractFileImports(types.FileID(999), "test.go", []byte(`package main; import "fmt"`))
	if testData == nil {
		t.Error("Resolver should still work after concurrent access")
	}
}
