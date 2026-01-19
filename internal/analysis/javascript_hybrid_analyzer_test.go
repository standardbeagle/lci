package analysis

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJavaScriptHybridAnalyzer_GetLanguageName(t *testing.T) {
	analyzer := NewJavaScriptHybridAnalyzer()
	assert.Equal(t, "javascript", analyzer.GetLanguageName())
}

func TestJavaScriptHybridAnalyzer_UsesGoFastForES5(t *testing.T) {
	analyzer := NewJavaScriptHybridAnalyzer()
	// ES5/CommonJS code that go-fAST can parse
	code := `
function greet(name) {
    return "Hello, " + name;
}

var add = function(a, b) {
    return a + b;
};

class Animal {
    constructor(name) {
        this.name = name;
    }
}
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, sym := range symbols {
		names[sym.Identity.Name] = true
	}

	assert.True(t, names["greet"], "Should find greet function")
	assert.True(t, names["Animal"], "Should find Animal class")
}

func TestJavaScriptHybridAnalyzer_FallsBackToRegexForES6Modules(t *testing.T) {
	analyzer := NewJavaScriptHybridAnalyzer()
	// ES6 module code that go-fAST can't parse
	code := `
import { something } from 'module';

export function createUser(name) {
    return { name, id: Math.random() };
}

export class UserService {
    getUser(id) {
        return fetch('/api/users/' + id);
    }
}
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, sym := range symbols {
		names[sym.Identity.Name] = true
	}

	// Should still find symbols via regex fallback
	assert.True(t, names["createUser"], "Should find createUser function via regex fallback")
	assert.True(t, names["UserService"], "Should find UserService class via regex fallback")
}

func TestJavaScriptHybridAnalyzer_HandlesMixedCode(t *testing.T) {
	analyzer := NewJavaScriptHybridAnalyzer()
	// JSX code (also needs regex fallback)
	code := `
class UserComponent extends React.Component {
    constructor(props) {
        super(props);
        this.state = { name: '' };
    }

    render() {
        return <div>{this.state.name}</div>;
    }
}

export function createUser(name) {
    return { name, id: Math.random() };
}
`
	fileID := types.FileID(1)
	symbols, err := analyzer.ExtractSymbols(fileID, code, "test.js")
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, sym := range symbols {
		names[sym.Identity.Name] = true
	}

	// Should find symbols via regex fallback (JSX not supported by go-fAST)
	assert.True(t, names["UserComponent"], "Should find UserComponent class")
	assert.True(t, names["createUser"], "Should find createUser function")
}
