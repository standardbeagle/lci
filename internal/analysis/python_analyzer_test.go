package analysis

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonAnalyzer_GetLanguageName(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	assert.Equal(t, "python", analyzer.GetLanguageName())
}

func TestPythonAnalyzer_ExtractSymbols_Functions(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
def greet(name):
    return f"Hello, {name}"

def add(a, b):
    return a + b

async def fetch_data(url):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            return await response.json()
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, sym := range symbols {
		names[sym.Identity.Name] = true
	}

	assert.True(t, names["greet"], "Should find greet function")
	assert.True(t, names["add"], "Should find add function")
	assert.True(t, names["fetch_data"], "Should find fetch_data async function")
}

func TestPythonAnalyzer_ExtractSymbols_Classes(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
class Animal:
    def __init__(self, name):
        self.name = name

    def speak(self):
        print(f"{self.name} makes a sound")

class Dog(Animal):
    def speak(self):
        print(f"{self.name} barks")

    @classmethod
    def create_puppy(cls, name):
        return cls(name)
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)

	// Categorize symbols
	var classes []*types.UniversalSymbolNode
	var methods []*types.UniversalSymbolNode

	for _, sym := range symbols {
		switch sym.Identity.Kind {
		case types.SymbolKindClass:
			classes = append(classes, sym)
		case types.SymbolKindMethod:
			methods = append(methods, sym)
		}
	}

	assert.Equal(t, 2, len(classes), "Should find exactly 2 classes (Animal, Dog)")
	assert.GreaterOrEqual(t, len(methods), 4, "Should find at least 4 methods")
}

func TestPythonAnalyzer_ExtractSymbols_Decorators(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
@dataclass
class Config:
    debug: bool = False
    host: str = "localhost"

@app.route("/api/users")
@require_auth
def get_users():
    return User.all()

@staticmethod
def helper():
    pass
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)

	// Find symbols and check decorators
	for _, sym := range symbols {
		if sym.Identity.Name == "Config" {
			hasDataclass := false
			for _, attr := range sym.Metadata.Attributes {
				if attr.Type == types.AttrTypeDecorator && attr.Value == "@dataclass" {
					hasDataclass = true
				}
			}
			assert.True(t, hasDataclass, "Config class should have @dataclass decorator")
		}

		if sym.Identity.Name == "get_users" {
			decoratorCount := 0
			for _, attr := range sym.Metadata.Attributes {
				if attr.Type == types.AttrTypeDecorator {
					decoratorCount++
				}
			}
			assert.GreaterOrEqual(t, decoratorCount, 2, "get_users should have at least 2 decorators")
		}
	}
}

func TestPythonAnalyzer_ExtractSymbols_AccessLevels(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
class Example:
    def public_method(self):
        pass

    def _protected_method(self):
        pass

    def __private_method(self):
        pass

    def __dunder_method__(self):
        pass
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)

	accessLevels := make(map[string]types.AccessLevel)
	for _, sym := range symbols {
		accessLevels[sym.Identity.Name] = sym.Visibility.Access
	}

	assert.Equal(t, types.AccessPublic, accessLevels["public_method"], "public_method should be public")
	assert.Equal(t, types.AccessProtected, accessLevels["_protected_method"], "_protected_method should be protected")
	assert.Equal(t, types.AccessPrivate, accessLevels["__private_method"], "__private_method should be private")
	assert.Equal(t, types.AccessPublic, accessLevels["__dunder_method__"], "__dunder_method__ should be public (dunder)")
}

func TestPythonAnalyzer_AnalyzeDependencies(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
import os
import sys
from pathlib import Path
from typing import List, Dict
from collections import defaultdict

def read_config():
    pass
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)
	require.NotEmpty(t, symbols)

	deps, err := analyzer.AnalyzeDependencies(symbols[0], code, "test.py")
	require.NoError(t, err)

	// Should find multiple import dependencies
	assert.GreaterOrEqual(t, len(deps), 4, "Should find at least 4 import dependencies")

	// Check import paths
	importPaths := make(map[string]bool)
	for _, dep := range deps {
		importPaths[dep.ImportPath] = true
	}

	assert.True(t, importPaths["os"], "Should find os import")
	assert.True(t, importPaths["sys"], "Should find sys import")
}

func TestPythonAnalyzer_AnalyzeCalls(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
def helper(x):
    return x * 2

def main():
    result = helper(5)
    print(result)
    data = list(range(10))
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)

	// Find the main function
	var mainFunc *types.UniversalSymbolNode
	for _, sym := range symbols {
		if sym.Identity.Name == "main" && sym.Identity.Kind == types.SymbolKindFunction {
			mainFunc = sym
			break
		}
	}
	require.NotNil(t, mainFunc, "Should find main function")

	calls, err := analyzer.AnalyzeCalls(mainFunc, code, "test.py")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(calls), 1, "Should find at least 1 function call in main")
}

func TestPythonAnalyzer_AnalyzeExtends(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
class Base:
    pass

class Derived(Base):
    pass

class Multiple(Base, Mixin):
    pass

class WithGeneric(List[str]):
    pass
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)

	for _, sym := range symbols {
		if sym.Identity.Name == "Derived" {
			extends, err := analyzer.AnalyzeExtends(sym, code, "test.py")
			require.NoError(t, err)
			assert.Equal(t, 1, len(extends), "Derived should extend 1 class")
		}

		if sym.Identity.Name == "Multiple" {
			extends, err := analyzer.AnalyzeExtends(sym, code, "test.py")
			require.NoError(t, err)
			assert.Equal(t, 2, len(extends), "Multiple should extend 2 classes")
		}
	}
}

func TestPythonAnalyzer_AsyncFunction(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
async def fetch(url):
    pass

def sync_func():
    pass
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)

	for _, sym := range symbols {
		if sym.Identity.Name == "fetch" {
			hasAsync := false
			for _, attr := range sym.Metadata.Attributes {
				if attr.Type == types.AttrTypeAsync {
					hasAsync = true
				}
			}
			assert.True(t, hasAsync, "fetch should have async attribute")
		}

		if sym.Identity.Name == "sync_func" {
			hasAsync := false
			for _, attr := range sym.Metadata.Attributes {
				if attr.Type == types.AttrTypeAsync {
					hasAsync = true
				}
			}
			assert.False(t, hasAsync, "sync_func should not have async attribute")
		}
	}
}

func TestPythonAnalyzer_MethodContainment(t *testing.T) {
	analyzer := NewPythonAnalyzer()
	code := `
class MyClass:
    def method_one(self):
        pass

    def method_two(self):
        pass

def standalone_func():
    pass
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.py")
	require.NoError(t, err)

	// Find the class and methods
	var classSymbol *types.UniversalSymbolNode
	methodsContained := 0

	for _, sym := range symbols {
		if sym.Identity.Name == "MyClass" {
			classSymbol = sym
		}
		if sym.Relationships.ContainedBy != nil {
			methodsContained++
		}
	}

	require.NotNil(t, classSymbol, "Should find MyClass")
	assert.Equal(t, 2, len(classSymbol.Relationships.Contains), "MyClass should contain 2 methods")
	assert.Equal(t, 2, methodsContained, "2 methods should have ContainedBy set")
}
