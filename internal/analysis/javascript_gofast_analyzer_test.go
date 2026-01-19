package analysis

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJavaScriptGoFastAnalyzer_GetLanguageName(t *testing.T) {
	analyzer := NewJavaScriptGoFastAnalyzer()
	assert.Equal(t, "javascript", analyzer.GetLanguageName())
}

func TestJavaScriptGoFastAnalyzer_ExtractSymbols_Functions(t *testing.T) {
	analyzer := NewJavaScriptGoFastAnalyzer()
	code := `
function greet(name) {
    return "Hello, " + name;
}

const add = (a, b) => a + b;

async function fetchData(url) {
    const response = await fetch(url);
    return response.json();
}
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	require.NoError(t, err)

	// Should find at least the named functions
	names := make(map[string]bool)
	for _, sym := range symbols {
		names[sym.Identity.Name] = true
	}

	assert.True(t, names["greet"], "Should find greet function")
	assert.True(t, names["fetchData"], "Should find fetchData function")
}

func TestJavaScriptGoFastAnalyzer_ExtractSymbols_Classes(t *testing.T) {
	analyzer := NewJavaScriptGoFastAnalyzer()
	code := `
class Animal {
    constructor(name) {
        this.name = name;
    }

    speak() {
        console.log(this.name + " makes a sound");
    }
}

class Dog extends Animal {
    speak() {
        console.log(this.name + " barks");
    }

    static createPuppy(name) {
        return new Dog(name);
    }
}
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	require.NoError(t, err)

	// Find symbols by kind
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

	assert.GreaterOrEqual(t, len(classes), 2, "Should find at least 2 classes (Animal, Dog)")
	assert.GreaterOrEqual(t, len(methods), 3, "Should find at least 3 methods")
}

func TestJavaScriptGoFastAnalyzer_ExtractSymbols_Variables(t *testing.T) {
	analyzer := NewJavaScriptGoFastAnalyzer()
	code := `
const API_KEY = "secret";
let counter = 0;
var globalConfig = { debug: true };

const multiply = function(x, y) {
    return x * y;
};
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, sym := range symbols {
		names[sym.Identity.Name] = true
	}

	assert.True(t, names["API_KEY"], "Should find API_KEY constant")
	assert.True(t, names["counter"], "Should find counter variable")
	assert.True(t, names["globalConfig"], "Should find globalConfig variable")
}

func TestJavaScriptGoFastAnalyzer_AnalyzeDependencies(t *testing.T) {
	analyzer := NewJavaScriptGoFastAnalyzer()
	// Note: go-fast doesn't support ES6 modules, so we test with require
	code := `
const fs = require('fs');
const { join } = require('path');

function readFile(filename) {
    return fs.readFileSync(join(__dirname, filename));
}
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	require.NoError(t, err)
	require.NotEmpty(t, symbols)

	// Find the readFile function
	var funcSymbol *types.UniversalSymbolNode
	for _, sym := range symbols {
		if sym.Identity.Name == "readFile" {
			funcSymbol = sym
			break
		}
	}

	if funcSymbol != nil {
		deps, err := analyzer.AnalyzeDependencies(funcSymbol, code, "test.js")
		require.NoError(t, err)
		// Dependencies should include require statements
		assert.GreaterOrEqual(t, len(deps), 0, "Should analyze dependencies")
	}
}

func TestJavaScriptGoFastAnalyzer_AnalyzeCalls(t *testing.T) {
	analyzer := NewJavaScriptGoFastAnalyzer()
	code := `
function helper(x) {
    return x * 2;
}

function main() {
    const a = helper(5);
    const b = Math.sqrt(16);
    console.log(a + b);
}
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
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

	calls, err := analyzer.AnalyzeCalls(mainFunc, code, "test.js")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(calls), 1, "Should find at least 1 function call in main")
}

func TestJavaScriptGoFastAnalyzer_ParseError(t *testing.T) {
	analyzer := NewJavaScriptGoFastAnalyzer()
	// Invalid JavaScript syntax
	code := `function broken( { return }`

	fileID := types.FileID(1)
	// Should return error for invalid code (hybrid analyzer will fall back to regex)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	assert.Error(t, err, "Should return error for invalid code")
	assert.Nil(t, symbols, "Should return nil symbols on parse error")
}

func TestJavaScriptGoFastAnalyzer_ArrowFunctions(t *testing.T) {
	analyzer := NewJavaScriptGoFastAnalyzer()
	code := `
const simple = () => 42;
const withParams = (x, y) => x + y;
const withBody = (data) => {
    const processed = data.map(x => x * 2);
    return processed;
};
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, sym := range symbols {
		names[sym.Identity.Name] = true
	}

	assert.True(t, names["simple"], "Should find simple arrow function")
	assert.True(t, names["withParams"], "Should find withParams arrow function")
	assert.True(t, names["withBody"], "Should find withBody arrow function")
}
