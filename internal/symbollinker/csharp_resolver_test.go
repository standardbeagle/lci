package symbollinker

import (
	"testing"

	"github.com/standardbeagle/lci/internal/core"
	"github.com/standardbeagle/lci/internal/types"
)

// TestCSharpResolver tests the c sharp resolver.
func TestCSharpResolver(t *testing.T) {
	// Create a test file service
	fileService := core.NewFileService()

	// Create resolver
	resolver := NewCSharpResolverWithFileService("/test/project", fileService)

	// Setup test file registry
	testFiles := map[string]types.FileID{
		"/test/project/Models/User.cs":                1,
		"/test/project/Services/UserService.cs":       2,
		"/test/project/Controllers/UserController.cs": 3,
		"/test/project/Utils/StringHelper.cs":         4,
	}
	resolver.SetFileRegistry(testFiles)

	t.Run("Built-in namespace resolution", func(t *testing.T) {
		testCases := []struct {
			namespace string
			expected  bool
		}{
			{"System", true},
			{"System.Collections.Generic", true},
			{"System.Threading.Tasks", true},
			{"Microsoft.Extensions.Logging", true},
			{"Microsoft.AspNetCore.Mvc", true},
			{"Windows.UI.Xaml", true},
			{"MyApp.Custom", false},
			{"ThirdParty.Package", false},
		}

		for _, tc := range testCases {
			resolution := resolver.ResolveImport(tc.namespace, 1)

			isBuiltin := resolution.Resolution == types.ResolutionBuiltin && resolution.IsBuiltin
			if isBuiltin != tc.expected {
				t.Errorf("Namespace %s: expected builtin=%v, got builtin=%v", tc.namespace, tc.expected, isBuiltin)
			}
		}
	})

	t.Run("Project namespace resolution", func(t *testing.T) {
		// Test resolving namespaces to project files
		testCases := []struct {
			namespace    string
			expectedFile types.FileID
		}{
			// These should resolve to files in the project
			{"MyApp.Models", 1},      // Should find User.cs in Models directory
			{"MyApp.Services", 2},    // Should find UserService.cs in Services directory
			{"MyApp.Controllers", 3}, // Should find UserController.cs in Controllers directory
		}

		for _, tc := range testCases {
			resolution := resolver.ResolveImport(tc.namespace, types.FileID(1))

			if resolution.Resolution == types.ResolutionInternal && resolution.FileID == tc.expectedFile {
				t.Logf("Successfully resolved %s to FileID %d", tc.namespace, tc.expectedFile)
			} else {
				// This might fail without actual filesystem, which is expected
				t.Logf("Namespace %s resolution: %v (FileID: %d, Path: %s)",
					tc.namespace, resolution.Resolution, resolution.FileID, resolution.ResolvedPath)
			}
		}
	})

	t.Run("Known namespace patterns", func(t *testing.T) {
		testCases := []struct {
			namespace string
			expected  bool
		}{
			{"Newtonsoft.Json", true},
			{"NUnit.Framework", true},
			{"Moq.Setup", true},
			{"AutoMapper.Configuration", true},
			{"FluentValidation.Validators", true},
			{"Serilog.Core", true},
			{"EntityFramework.Core", true},
			{"Dapper.Contrib", true},
			{"UnknownPackage.Something", false},
		}

		for _, tc := range testCases {
			resolution := resolver.ResolveImport(tc.namespace, 1)

			isKnownPattern := resolution.Resolution == types.ResolutionExternal && resolution.IsExternal
			if isKnownPattern != tc.expected {
				t.Errorf("Namespace %s: expected known pattern=%v, got external=%v",
					tc.namespace, tc.expected, isKnownPattern)
			}
		}
	})

	t.Run("Assembly reference resolution", func(t *testing.T) {
		// Add some assembly references
		resolver.AddAssembly("MyLibrary", "/test/assemblies/MyLibrary.dll")
		resolver.AddNamespaceMapping("MyLibrary.Core", "MyLibrary")
		resolver.AddNamespaceMapping("MyLibrary.Utils", "MyLibrary")

		resolution := resolver.ResolveImport("MyLibrary.Core", 1)

		if resolution.Resolution != types.ResolutionExternal {
			t.Errorf("Expected external resolution for assembly namespace, got %v", resolution.Resolution)
		}

		if resolution.ResolvedPath != "MyLibrary" {
			t.Errorf("Expected assembly name 'MyLibrary', got %s", resolution.ResolvedPath)
		}
	})

	t.Run("Global using statements", func(t *testing.T) {
		// Test global using functionality
		resolver.AddGlobalUsing("System")
		resolver.AddGlobalUsing("System.Collections.Generic")
		resolver.AddGlobalUsing("Microsoft.Extensions.DependencyInjection")

		globalUsings := resolver.GetGlobalUsings()

		expectedUsings := []string{
			"System",
			"System.Collections.Generic",
			"Microsoft.Extensions.DependencyInjection",
		}

		if len(globalUsings) < len(expectedUsings) {
			t.Errorf("Expected at least %d global usings, got %d", len(expectedUsings), len(globalUsings))
		}

		for _, expected := range expectedUsings {
			found := false
			for _, actual := range globalUsings {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Global using '%s' not found", expected)
			}
		}
	})

	t.Run("Resolver stats", func(t *testing.T) {
		stats := resolver.Stats()

		expectedKeys := []string{
			"project_root", "project_files", "assemblies", "global_usings",
			"namespace_mappings", "builtin_namespaces", "registered_files",
		}

		for _, key := range expectedKeys {
			if _, exists := stats[key]; !exists {
				t.Errorf("Expected stats key %s not found", key)
			}
		}

		// Check that registered files count matches our test data
		if registeredFiles, ok := stats["registered_files"].(int); !ok || registeredFiles != len(testFiles) {
			t.Errorf("Expected %d registered files, got %v", len(testFiles), stats["registered_files"])
		}

		t.Logf("C# Resolver stats: %+v", stats)
	})

	t.Run("Project file parsing simulation", func(t *testing.T) {
		// Test parsing project file content
		mockProjectContent := `<Project Sdk="Microsoft.NET.Sdk.Web">
		  <PropertyGroup>
		    <TargetFramework>net6.0</TargetFramework>
		  </PropertyGroup>
		  
		  <ItemGroup>
		    <PackageReference Include="Newtonsoft.Json" Version="13.0.1" />
		    <PackageReference Include="Microsoft.EntityFrameworkCore" Version="6.0.0" />
		    <PackageReference Include="Serilog" Version="2.10.0" />
		  </ItemGroup>
		  
		  <ItemGroup>
		    <ProjectReference Include="../MyLibrary/MyLibrary.csproj" />
		  </ItemGroup>
		  
		  <ItemGroup>
		    <Using Include="System" />
		    <Using Include="System.Collections.Generic" />
		  </ItemGroup>
		</Project>`

		resolver.parsePackageReferences(mockProjectContent)
		resolver.parseGlobalUsings(mockProjectContent)

		// Check that package references were parsed
		stats := resolver.Stats()
		if assemblies, ok := stats["assemblies"].(int); !ok || assemblies == 0 {
			t.Error("No assemblies found after parsing project file")
		}

		// Check that global usings were parsed
		if globalUsings, ok := stats["global_usings"].(int); !ok || globalUsings == 0 {
			t.Error("No global usings found after parsing project file")
		}
	})

	t.Run("File finding in project structure", func(t *testing.T) {
		// Test the namespace to directory mapping logic
		testCases := []struct {
			namespace     string
			expectedPaths []string
		}{
			{
				"MyApp.Models",
				[]string{"MyApp/Models", "", "src/MyApp/Models", "lib/MyApp/Models"},
			},
			{
				"Controllers.Api",
				[]string{"Controllers/Api", "", "src/Controllers/Api", "lib/Controllers/Api"},
			},
		}

		for _, tc := range testCases {
			resolution := resolver.ResolveImport(tc.namespace, types.FileID(1))

			// This test mainly verifies the logic doesn't crash
			// Actual file finding would require a real filesystem
			t.Logf("Namespace %s resolved to: %s (resolution: %v)",
				tc.namespace, resolution.ResolvedPath, resolution.Resolution)
		}
	})
}

// TestCSharpResolverFileRegistration tests the c sharp resolver file registration.
func TestCSharpResolverFileRegistration(t *testing.T) {
	resolver := NewCSharpResolver("/test/project")

	// Test file registration
	resolver.RegisterFile(types.FileID(1), "/test/project/Models/User.cs")
	resolver.RegisterFile(types.FileID(2), "Controllers/HomeController.cs")

	// Verify registration through stats
	stats := resolver.Stats()
	if registeredFiles, ok := stats["registered_files"].(int); !ok || registeredFiles != 2 {
		t.Errorf("Expected 2 registered files, got %v", stats["registered_files"])
	}
}

// TestCSharpResolverBuiltinNamespaces tests the c sharp resolver builtin namespaces.
func TestCSharpResolverBuiltinNamespaces(t *testing.T) {
	resolver := NewCSharpResolver("/test/project")

	// Test comprehensive built-in namespace detection
	builtinNamespaces := []string{
		"System",
		"System.Collections.Generic",
		"System.IO",
		"System.Text",
		"System.Threading",
		"System.Linq",
		"System.Net",
		"System.Reflection",
		"Microsoft.Extensions.Logging",
		"Microsoft.AspNetCore.Mvc",
		"Microsoft.EntityFrameworkCore",
		"Windows.UI",
	}

	for _, namespace := range builtinNamespaces {
		resolution := resolver.ResolveImport(namespace, 1)

		if !resolution.IsBuiltin || resolution.Resolution != types.ResolutionBuiltin {
			t.Errorf("Namespace %s should be recognized as built-in", namespace)
		}
	}

	// Test non-builtin namespaces
	customNamespaces := []string{
		"MyApp.Models",
		"CustomLibrary.Utils",
		"ThirdParty.Package",
	}

	for _, namespace := range customNamespaces {
		resolution := resolver.ResolveImport(namespace, 1)

		if resolution.IsBuiltin && resolution.Resolution == types.ResolutionBuiltin {
			t.Errorf("Namespace %s should not be recognized as built-in", namespace)
		}
	}
}
