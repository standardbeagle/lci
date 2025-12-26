package symbollinker

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestGoResolver tests the go resolver.
func TestGoResolver(t *testing.T) {
	// Get absolute path to test project
	testProjectPath, err := filepath.Abs("testdata/go_project")
	if err != nil {
		t.Fatalf("Failed to get test project path: %v", err)
	}

	resolver := NewGoResolver(testProjectPath)

	t.Run("Module detection", func(t *testing.T) {
		if resolver.GetModuleName() != "github.com/example/testproject" {
			t.Errorf("Expected module name 'github.com/example/testproject', got '%s'", resolver.GetModuleName())
		}

		if resolver.goModPath == "" {
			t.Error("Expected go.mod path to be found")
		}
	})

	t.Run("File registration", func(t *testing.T) {
		// Register test files
		mainPath := filepath.Join(testProjectPath, "main.go")
		utilsPath := filepath.Join(testProjectPath, "pkg/utils/helper.go")
		configPath := filepath.Join(testProjectPath, "internal/config/config.go")

		resolver.RegisterFile(types.FileID(1), mainPath)
		resolver.RegisterFile(types.FileID(2), utilsPath)
		resolver.RegisterFile(types.FileID(3), configPath)

		// Verify registration
		if resolver.fileIDToPath[types.FileID(1)] != mainPath {
			t.Error("File registration failed for main.go")
		}
	})

	t.Run("Standard library resolution", func(t *testing.T) {
		tests := []struct {
			importPath string
			isBuiltin  bool
		}{
			{"fmt", true},
			{"os", true},
			{"strings", true},
			{"encoding/json", true},
			{"github.com/example/testproject", false},
			{"github.com/stretchr/testify", false},
		}

		for _, test := range tests {
			resolution := resolver.ResolveImport(test.importPath, types.FileID(1))

			if resolution.IsBuiltin != test.isBuiltin {
				t.Errorf("Import %s: expected IsBuiltin=%v, got %v",
					test.importPath, test.isBuiltin, resolution.IsBuiltin)
			}

			if test.isBuiltin && resolution.Resolution != types.ResolutionBuiltin {
				t.Errorf("Import %s: expected ResolutionBuiltin, got %v",
					test.importPath, resolution.Resolution)
			}
		}
	})

	t.Run("Module import resolution", func(t *testing.T) {
		// Register the utils file first
		utilsPath := filepath.Join(testProjectPath, "pkg/utils/helper.go")
		resolver.RegisterFile(types.FileID(2), utilsPath)

		// Test resolving module imports
		resolution := resolver.ResolveImport("github.com/example/testproject/pkg/utils", types.FileID(1))

		if resolution.Resolution != types.ResolutionModule && resolution.Resolution != types.ResolutionDirectory {
			t.Errorf("Expected module/directory resolution, got %v", resolution.Resolution)
		}

		if resolution.IsExternal {
			t.Error("Module import should not be marked as external")
		}

		expectedPath := filepath.Join(testProjectPath, "pkg/utils")
		if resolution.ResolvedPath != expectedPath {
			t.Errorf("Expected resolved path %s, got %s", expectedPath, resolution.ResolvedPath)
		}
	})

	t.Run("Internal package resolution", func(t *testing.T) {
		// Register the config file
		configPath := filepath.Join(testProjectPath, "internal/config/config.go")
		resolver.RegisterFile(types.FileID(3), configPath)

		// Test resolving internal package
		resolution := resolver.ResolveImport("github.com/example/testproject/internal/config", types.FileID(1))

		if resolution.Resolution != types.ResolutionModule && resolution.Resolution != types.ResolutionDirectory {
			t.Errorf("Expected module/directory resolution for internal package, got %v", resolution.Resolution)
		}

		// Internal packages should be found but have special visibility rules
		// (not implemented in this basic version)
		if resolution.IsExternal {
			t.Error("Internal package should not be marked as external")
		}
	})

	t.Run("External package resolution", func(t *testing.T) {
		// Test external package resolution
		resolution := resolver.ResolveImport("github.com/stretchr/testify/assert", types.FileID(1))

		if !resolution.IsExternal {
			t.Error("Expected external package to be marked as external")
		}

		if resolution.Resolution != types.ResolutionExternal {
			t.Errorf("Expected ResolutionExternal, got %v", resolution.Resolution)
		}

		if resolution.RequestPath != "github.com/stretchr/testify/assert" {
			t.Errorf("RequestPath should be preserved for external packages")
		}
	})

	t.Run("Subpackage resolution", func(t *testing.T) {
		// Register subpackage file
		subPath := filepath.Join(testProjectPath, "pkg/utils/subpkg/sub.go")
		resolver.RegisterFile(types.FileID(4), subPath)

		// Test resolving subpackage
		resolution := resolver.ResolveImport("github.com/example/testproject/pkg/utils/subpkg", types.FileID(2))

		if resolution.IsExternal {
			t.Error("Subpackage should not be marked as external")
		}

		expectedPath := filepath.Join(testProjectPath, "pkg/utils/subpkg")
		if resolution.ResolvedPath != expectedPath {
			t.Errorf("Expected resolved path %s, got %s", expectedPath, resolution.ResolvedPath)
		}
	})

	t.Run("Non-existent import resolution", func(t *testing.T) {
		resolution := resolver.ResolveImport("github.com/example/testproject/nonexistent", types.FileID(1))

		if resolution.Resolution != types.ResolutionNotFound {
			t.Errorf("Expected ResolutionNotFound for non-existent package, got %v", resolution.Resolution)
		}
	})
}

// TestGoResolverRelativeImports tests the go resolver relative imports.
func TestGoResolverRelativeImports(t *testing.T) {
	// This would test relative imports like "./subpkg" or "../other"
	// In practice, Go doesn't use relative imports in module mode,
	// but they're supported in GOPATH mode

	testProjectPath, err := filepath.Abs("testdata/go_project")
	if err != nil {
		t.Fatalf("Failed to get test project path: %v", err)
	}

	resolver := NewGoResolver(testProjectPath)

	// Register files
	utilsPath := filepath.Join(testProjectPath, "pkg/utils/helper.go")
	subPath := filepath.Join(testProjectPath, "pkg/utils/subpkg/sub.go")

	resolver.RegisterFile(types.FileID(1), utilsPath)
	resolver.RegisterFile(types.FileID(2), subPath)

	t.Run("Relative import with ./", func(t *testing.T) {
		// From utils/helper.go, import "./subpkg"
		resolution := resolver.ResolveImport("./subpkg", types.FileID(1))

		if resolution.Resolution == types.ResolutionNotFound {
			t.Error("Should resolve relative import ./subpkg")
		}

		expectedPath := filepath.Join(testProjectPath, "pkg/utils/subpkg")
		if resolution.ResolvedPath != expectedPath && !strings.HasPrefix(resolution.ResolvedPath, expectedPath) {
			t.Errorf("Expected path to contain %s, got %s", expectedPath, resolution.ResolvedPath)
		}
	})
}

// TestGoResolverPackagePath tests the go resolver package path.
func TestGoResolverPackagePath(t *testing.T) {
	testProjectPath, err := filepath.Abs("testdata/go_project")
	if err != nil {
		t.Fatalf("Failed to get test project path: %v", err)
	}

	resolver := NewGoResolver(testProjectPath)

	tests := []struct {
		pkgPath  string
		expected string
	}{
		{"fmt", "fmt"},                     // Standard library
		{"encoding/json", "encoding/json"}, // Standard library subpackage
		{"github.com/example/testproject/pkg/utils", filepath.Join(testProjectPath, "pkg/utils")},
		{"github.com/stretchr/testify", "github.com/stretchr/testify"}, // External
	}

	for _, test := range tests {
		result := resolver.ResolvePackagePath(test.pkgPath)
		if result != test.expected {
			t.Errorf("ResolvePackagePath(%s): expected %s, got %s",
				test.pkgPath, test.expected, result)
		}
	}
}

// TestGoResolverNoGoMod tests the go resolver no go mod.
func TestGoResolverNoGoMod(t *testing.T) {
	// Test resolver behavior when there's no go.mod file
	tempDir := t.TempDir()

	resolver := NewGoResolver(tempDir)

	if resolver.GetModuleName() != "" {
		t.Errorf("Expected empty module name without go.mod, got '%s'", resolver.GetModuleName())
	}

	// Should still resolve standard library
	resolution := resolver.ResolveImport("fmt", types.FileID(1))
	if !resolution.IsBuiltin {
		t.Error("Should still resolve standard library without go.mod")
	}

	// Non-standard imports should be marked as external
	resolution = resolver.ResolveImport("github.com/example/package", types.FileID(1))
	if !resolution.IsExternal {
		t.Error("Non-standard imports should be external without go.mod")
	}
}
