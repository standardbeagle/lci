package git

import (
	"testing"
)

func TestDetectCaseStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected CaseStyle
	}{
		{"camelCase simple", "getUserName", CaseStyleCamelCase},
		{"camelCase single word", "user", CaseStyleUnknown},
		{"PascalCase simple", "GetUserName", CaseStylePascalCase},
		{"PascalCase class", "UserService", CaseStylePascalCase},
		{"snake_case simple", "get_user_name", CaseStyleSnakeCase},
		{"snake_case constant", "MAX_RETRY_COUNT", CaseStyleSnakeCase},
		{"kebab-case simple", "get-user-name", CaseStyleKebabCase},
		{"empty string", "", CaseStyleUnknown},
		{"single char", "x", CaseStyleUnknown},
		{"all lowercase", "username", CaseStyleUnknown},
		{"all uppercase", "USERNAME", CaseStyleUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectCaseStyle(tt.input)
			if result != tt.expected {
				t.Errorf("DetectCaseStyle(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCategorizeDiffSize(t *testing.T) {
	tests := []struct {
		name     string
		files    []ChangedFile
		expected DiffSize
	}{
		{"empty", nil, DiffSizeSmall},
		{"small - 5 files", make([]ChangedFile, 5), DiffSizeSmall},
		{"small - 9 files", make([]ChangedFile, 9), DiffSizeSmall},
		{"medium - 10 files", make([]ChangedFile, 10), DiffSizeMedium},
		{"medium - 50 files", make([]ChangedFile, 50), DiffSizeMedium},
		{"large - 51 files", make([]ChangedFile, 51), DiffSizeLarge},
		{"large - 100 files", make([]ChangedFile, 100), DiffSizeLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CategorizeDiffSize(tt.files)
			if result != tt.expected {
				t.Errorf("CategorizeDiffSize() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultAnalysisParams(t *testing.T) {
	params := DefaultAnalysisParams()

	if params.Scope != ScopeStaged {
		t.Errorf("DefaultAnalysisParams().Scope = %v, want %v", params.Scope, ScopeStaged)
	}

	if len(params.Focus) != 3 {
		t.Errorf("DefaultAnalysisParams().Focus length = %d, want 3 (duplicates, naming, metrics)", len(params.Focus))
	}

	if params.SimilarityThreshold != 0.8 {
		t.Errorf("DefaultAnalysisParams().SimilarityThreshold = %v, want 0.8", params.SimilarityThreshold)
	}

	if params.MaxFindings != 20 {
		t.Errorf("DefaultAnalysisParams().MaxFindings = %v, want 20", params.MaxFindings)
	}
}

func TestHasFocus(t *testing.T) {
	tests := []struct {
		name     string
		focus    []string
		check    string
		expected bool
	}{
		{"empty focus - duplicates", nil, "duplicates", true},
		{"empty focus - naming", nil, "naming", true},
		{"specific focus - match", []string{"duplicates"}, "duplicates", true},
		{"specific focus - no match", []string{"duplicates"}, "naming", false},
		{"all focus", []string{"all"}, "anything", true},
		{"multiple focus - match first", []string{"duplicates", "naming"}, "duplicates", true},
		{"multiple focus - match second", []string{"duplicates", "naming"}, "naming", true},
		{"multiple focus - no match", []string{"duplicates", "naming"}, "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := AnalysisParams{Focus: tt.focus}
			result := params.HasFocus(tt.check)
			if result != tt.expected {
				t.Errorf("HasFocus(%q) = %v, want %v", tt.check, result, tt.expected)
			}
		})
	}
}

func TestAnalysisScope(t *testing.T) {
	if ScopeStaged != "staged" {
		t.Errorf("ScopeStaged = %v, want %v", ScopeStaged, "staged")
	}
	if ScopeWIP != "wip" {
		t.Errorf("ScopeWIP = %v, want %v", ScopeWIP, "wip")
	}
	if ScopeCommit != "commit" {
		t.Errorf("ScopeCommit = %v, want %v", ScopeCommit, "commit")
	}
	if ScopeRange != "range" {
		t.Errorf("ScopeRange = %v, want %v", ScopeRange, "range")
	}
}

func TestFileChangeStatus(t *testing.T) {
	if FileStatusAdded != "added" {
		t.Errorf("FileStatusAdded = %v, want %v", FileStatusAdded, "added")
	}
	if FileStatusModified != "modified" {
		t.Errorf("FileStatusModified = %v, want %v", FileStatusModified, "modified")
	}
	if FileStatusDeleted != "deleted" {
		t.Errorf("FileStatusDeleted = %v, want %v", FileStatusDeleted, "deleted")
	}
	if FileStatusRenamed != "renamed" {
		t.Errorf("FileStatusRenamed = %v, want %v", FileStatusRenamed, "renamed")
	}
	if FileStatusCopied != "copied" {
		t.Errorf("FileStatusCopied = %v, want %v", FileStatusCopied, "copied")
	}
}

func TestGetLanguageFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected Language
	}{
		{"main.go", LangGo},
		{"path/to/file.go", LangGo},
		{"script.js", LangJavaScript},
		{"component.jsx", LangJavaScript},
		{"app.ts", LangTypeScript},
		{"component.tsx", LangTypeScript},
		{"main.py", LangPython},
		{"lib.rs", LangRust},
		{"Main.java", LangJava},
		{"Program.cs", LangCSharp},
		{"main.cpp", LangCpp},
		{"header.hpp", LangCpp},
		{"main.c", LangC},
		{"header.h", LangC},
		{"app.php", LangPHP},
		{"gem.rb", LangRuby},
		{"App.swift", LangSwift},
		{"Main.kt", LangKotlin},
		{"Main.scala", LangScala},
		{"main.zig", LangZig},
		{"unknown.xyz", LangUnknown},
		{"noextension", LangUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := GetLanguageFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("GetLanguageFromPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestSymbolTypeToKind(t *testing.T) {
	tests := []struct {
		symbolType string
		expected   SymbolKind
	}{
		{"function", KindFunction},
		{"method", KindMethod},
		{"class", KindClass},
		{"interface", KindInterface},
		{"struct", KindStruct},
		{"type", KindType},
		{"type_alias", KindType},
		{"constant", KindConstant},
		{"variable", KindVariable},
		{"field", KindField},
		{"enum", KindEnum},
		{"enum_member", KindEnumMember},
		{"module", KindModule},
		{"namespace", KindNamespace},
		{"property", KindProperty},
		{"unknown_type", KindUnknownKind},
	}

	for _, tt := range tests {
		t.Run(tt.symbolType, func(t *testing.T) {
			result := SymbolTypeToKind(tt.symbolType)
			if result != tt.expected {
				t.Errorf("SymbolTypeToKind(%q) = %v, want %v", tt.symbolType, result, tt.expected)
			}
		})
	}
}

func TestIsValidCaseStyle(t *testing.T) {
	tests := []struct {
		name     string
		lang     Language
		kind     SymbolKind
		style    CaseStyle
		expected bool
	}{
		// Go tests - allows both PascalCase and camelCase
		{"Go function PascalCase", LangGo, KindFunction, CaseStylePascalCase, true},
		{"Go function camelCase", LangGo, KindFunction, CaseStyleCamelCase, true},
		{"Go function snake_case", LangGo, KindFunction, CaseStyleSnakeCase, false},
		{"Go struct PascalCase", LangGo, KindStruct, CaseStylePascalCase, true},
		{"Go struct camelCase", LangGo, KindStruct, CaseStyleCamelCase, true},
		{"Go struct snake_case", LangGo, KindStruct, CaseStyleSnakeCase, false},

		// Python tests - snake_case for functions, PascalCase for classes
		{"Python function snake_case", LangPython, KindFunction, CaseStyleSnakeCase, true},
		{"Python function camelCase", LangPython, KindFunction, CaseStyleCamelCase, false},
		{"Python class PascalCase", LangPython, KindClass, CaseStylePascalCase, true},
		{"Python class snake_case", LangPython, KindClass, CaseStyleSnakeCase, false},

		// JavaScript tests - camelCase for functions, PascalCase for classes
		{"JS function camelCase", LangJavaScript, KindFunction, CaseStyleCamelCase, true},
		{"JS function PascalCase", LangJavaScript, KindFunction, CaseStylePascalCase, false},
		{"JS class PascalCase", LangJavaScript, KindClass, CaseStylePascalCase, true},
		{"JS class camelCase", LangJavaScript, KindClass, CaseStyleCamelCase, false},

		// Rust tests - snake_case for functions, PascalCase for structs
		{"Rust function snake_case", LangRust, KindFunction, CaseStyleSnakeCase, true},
		{"Rust function PascalCase", LangRust, KindFunction, CaseStylePascalCase, false},
		{"Rust struct PascalCase", LangRust, KindStruct, CaseStylePascalCase, true},
		{"Rust struct snake_case", LangRust, KindStruct, CaseStyleSnakeCase, false},

		// Unknown language accepts anything
		{"Unknown lang any style", LangUnknown, KindFunction, CaseStyleSnakeCase, true},

		// Unknown symbol kind for known language accepts anything
		{"Known lang unknown kind", LangGo, KindUnknownKind, CaseStyleSnakeCase, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidCaseStyle(tt.lang, tt.kind, tt.style)
			if result != tt.expected {
				t.Errorf("IsValidCaseStyle(%v, %v, %v) = %v, want %v",
					tt.lang, tt.kind, tt.style, result, tt.expected)
			}
		})
	}
}

func TestGetExpectedStyles(t *testing.T) {
	tests := []struct {
		name     string
		lang     Language
		kind     SymbolKind
		expected []CaseStyle
	}{
		{"Go function", LangGo, KindFunction, []CaseStyle{CaseStylePascalCase, CaseStyleCamelCase}},
		{"Python function", LangPython, KindFunction, []CaseStyle{CaseStyleSnakeCase}},
		{"Python class", LangPython, KindClass, []CaseStyle{CaseStylePascalCase}},
		{"Unknown lang", LangUnknown, KindFunction, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetExpectedStyles(tt.lang, tt.kind)
			if len(result) != len(tt.expected) {
				t.Errorf("GetExpectedStyles(%v, %v) = %v, want %v",
					tt.lang, tt.kind, result, tt.expected)
				return
			}
			for i, style := range result {
				if style != tt.expected[i] {
					t.Errorf("GetExpectedStyles(%v, %v)[%d] = %v, want %v",
						tt.lang, tt.kind, i, style, tt.expected[i])
				}
			}
		})
	}
}
