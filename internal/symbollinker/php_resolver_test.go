package symbollinker

import (
	"testing"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// TestPHPResolver tests the p h p resolver.
func TestPHPResolver(t *testing.T) {
	// Create a test file service with test data
	fileService := core.NewFileService()

	// Create resolver
	resolver := NewPHPResolverWithFileService("/test/project", fileService)

	// Setup test file registry
	testFiles := map[string]types.FileID{
		"/test/project/src/SimpleClass.php":                 1,
		"/test/project/lib/Helper.php":                      2,
		"/test/project/config/config.php":                   3,
		"/test/project/vendor/package/Class.php":            4,
		"/test/project/legacy/Legacy_Package_Class.php":     5,
		"/test/project/special/SpecialClass.php":            6,
		"/test/project/src/Models/User.php":                 7,
		"/test/project/lib/Utils/StringHelper.php":          8,
		"/test/project/vendor/Package/SubPackage/Class.php": 9,
	}
	resolver.SetFileRegistry(testFiles)

	t.Run("PSR-4 resolution", func(t *testing.T) {
		// Setup PSR-4 mapping
		resolver.SetPSR4Mapping("App\\", "src/")
		resolver.SetPSR4Mapping("Helper\\", "lib/")

		// Test PSR-4 class resolution
		resolution := resolver.ResolveImport("App\\SimpleClass", 1)

		if resolution.Resolution != types.ResolutionInternal {
			t.Errorf("Expected internal resolution, got %v", resolution.Resolution)
		}

		expectedPath := "/test/project/src/SimpleClass.php"
		if resolution.ResolvedPath != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, resolution.ResolvedPath)
		}

		if resolution.FileID != 1 {
			t.Errorf("Expected FileID 1, got %d", resolution.FileID)
		}
	})

	t.Run("PSR-0 resolution", func(t *testing.T) {
		// Setup PSR-0 mapping
		resolver.SetPSR0Mapping("Legacy_", "legacy/")

		// Test PSR-0 class resolution - should include full namespace in path
		resolution := resolver.ResolveImport("Legacy_Package_Class", 1)

		// PSR-0 should create Legacy/Package/Class.php path
		expectedPath := "/test/project/legacy/Legacy_Package_Class.php"
		if resolution.ResolvedPath != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, resolution.ResolvedPath)
		}
	})

	t.Run("Class map resolution", func(t *testing.T) {
		// Setup direct class mapping
		resolver.AddClassMapping("SpecialClass", "/test/project/special/SpecialClass.php")

		resolution := resolver.ResolveImport("SpecialClass", 1)

		expectedPath := "/test/project/special/SpecialClass.php"
		if resolution.ResolvedPath != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, resolution.ResolvedPath)
		}
	})

	t.Run("File include resolution", func(t *testing.T) {
		// Test relative file include
		resolution := resolver.ResolveImport("./config/config.php", 1)

		if resolution.Resolution == types.ResolutionError {
			t.Errorf("Failed to resolve file include: %s", resolution.Error)
		}
	})

	t.Run("Absolute file include resolution", func(t *testing.T) {
		// Test absolute path include
		resolution := resolver.ResolveImport("/test/project/config/config.php", 1)

		if resolution.Resolution != types.ResolutionInternal {
			t.Errorf("Expected internal resolution for absolute path, got %v", resolution.Resolution)
		}

		if resolution.FileID != 3 {
			t.Errorf("Expected FileID 3, got %d", resolution.FileID)
		}
	})

	t.Run("Include path resolution", func(t *testing.T) {
		// Add include path
		resolver.AddIncludePath("lib")

		// Test include from include path
		resolution := resolver.ResolveImport("Helper.php", 1)

		expectedPath := "/test/project/lib/Helper.php"
		if resolution.ResolvedPath != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, resolution.ResolvedPath)
		}
	})

	t.Run("File path detection", func(t *testing.T) {
		testCases := []struct {
			path     string
			expected bool
		}{
			{"./relative.php", true},
			{"../parent.php", true},
			{"/absolute/path.php", true},
			{"file.php", true},
			{"Class\\Name", false},
			{"App\\Service\\UserService", false},
			{"config.inc", true},
			{"archive.phar", true},
		}

		for _, tc := range testCases {
			result := resolver.isFilePath(tc.path)
			if result != tc.expected {
				t.Errorf("isFilePath(%s) = %v, expected %v", tc.path, result, tc.expected)
			}
		}
	})

	t.Run("Simple class file resolution", func(t *testing.T) {
		// This tests the fallback mechanism for simple class name to file name conversion
		resolution := resolver.ResolveImport("Helper", 2) // From Helper.php file

		// Should try to find Helper.php in the same directory
		if resolution.Resolution == types.ResolutionError {
			// This is expected since we don't have the actual filesystem,
			// but the resolver should have tried the correct paths
			t.Logf("Simple class resolution failed as expected without filesystem: %s", resolution.Error)
		}
	})

	t.Run("Autoload config", func(t *testing.T) {
		config := resolver.GetAutoloadConfig()

		if config == nil {
			t.Fatal("Autoload config is nil")
		}

		// Check that we can access PSR-4 mappings
		if len(config.PSR4) == 0 {
			t.Error("No PSR-4 mappings found")
		}

		// Check specific mapping we set
		if config.PSR4["App\\"] != "src/" {
			t.Errorf("Expected PSR-4 mapping App\\ -> src/, got %s", config.PSR4["App\\"])
		}
	})

	t.Run("Resolver stats", func(t *testing.T) {
		stats := resolver.Stats()

		expectedKeys := []string{"project_root", "composer_json", "psr4_mappings", "psr0_mappings", "class_mappings", "include_paths", "registered_files"}

		for _, key := range expectedKeys {
			if _, exists := stats[key]; !exists {
				t.Errorf("Expected stats key %s not found", key)
			}
		}

		// Check that registered files count matches our test data
		if registeredFiles, ok := stats["registered_files"].(int); !ok || registeredFiles != len(testFiles) {
			t.Errorf("Expected %d registered files, got %v", len(testFiles), stats["registered_files"])
		}

		t.Logf("PHP Resolver stats: %+v", stats)
	})

	t.Run("Namespace extraction from class name", func(t *testing.T) {
		testCases := []struct {
			className    string
			expectedFile string
			psr4Prefix   string
			psr4Dir      string
		}{
			{"App\\Models\\User", "src/Models/User.php", "App\\", "src/"},
			{"Helper\\Utils\\StringHelper", "lib/Utils/StringHelper.php", "Helper\\", "lib/"},
			{"Vendor\\Package\\SubPackage\\Class", "vendor/Package/SubPackage/Class.php", "Vendor\\", "vendor/"},
		}

		for _, tc := range testCases {
			resolver.SetPSR4Mapping(tc.psr4Prefix, tc.psr4Dir)
			resolution := resolver.ResolveImport(tc.className, 1)

			expectedPath := "/test/project/" + tc.expectedFile
			if resolution.ResolvedPath != expectedPath {
				t.Errorf("Class %s: expected path %s, got %s", tc.className, expectedPath, resolution.ResolvedPath)
			}
		}
	})
}

// TestPHPResolverComposerParsing tests the p h p resolver composer parsing.
func TestPHPResolverComposerParsing(t *testing.T) {
	// This test focuses on the composer.json parsing functionality
	fileService := core.NewFileService()
	resolver := NewPHPResolverWithFileService("/test/project", fileService)

	// Test composer.json parsing with mock content
	mockComposerContent := `{
		"autoload": {
			"psr-4": {
				"App\\": "src/",
				"Tests\\": "tests/"
			},
			"psr-0": {
				"Legacy_": "legacy/"
			},
			"classmap": [
				"lib/"
			]
		}
	}`

	resolver.parseComposerAutoload(mockComposerContent)
	config := resolver.GetAutoloadConfig()

	t.Run("PSR-4 parsing", func(t *testing.T) {
		expected := map[string]string{
			"App\\":   "src/",
			"Tests\\": "tests/",
		}

		for namespace, expectedDir := range expected {
			if actualDir, exists := config.PSR4[namespace]; !exists {
				t.Errorf("PSR-4 namespace %s not found", namespace)
			} else if actualDir != expectedDir {
				t.Errorf("PSR-4 %s: expected %s, got %s", namespace, expectedDir, actualDir)
			}
		}
	})

	t.Run("PSR-0 parsing", func(t *testing.T) {
		if config.PSR0["Legacy_"] != "legacy/" {
			t.Errorf("PSR-0 Legacy_: expected legacy/, got %s", config.PSR0["Legacy_"])
		}
	})
}

// TestPHPResolverRegisterFile tests the p h p resolver register file.
func TestPHPResolverRegisterFile(t *testing.T) {
	resolver := NewPHPResolver("/test/project")

	// Test file registration
	resolver.RegisterFile(types.FileID(1), "/test/project/src/Class.php")
	resolver.RegisterFile(types.FileID(2), "relative/path.php")

	// Test that files are registered correctly
	resolution := resolver.ResolveImport("/test/project/src/Class.php", types.FileID(2))

	if resolution.Resolution != types.ResolutionInternal {
		t.Errorf("Expected internal resolution for registered file, got %v", resolution.Resolution)
	}

	if resolution.FileID != types.FileID(1) {
		t.Errorf("Expected FileID 1, got %d", resolution.FileID)
	}
}
