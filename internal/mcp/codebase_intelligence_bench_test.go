package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/indexing"
	"github.com/standardbeagle/lci/internal/types"
)

// BenchmarkCodebaseIntelligence benchmarks the codebase intelligence system
func BenchmarkCodebaseIntelligence(b *testing.B) {
	// Create a test project with various sizes
	sizes := []struct {
		name         string
		numFiles     int
		linesPerFile int
	}{
		{"Small_10files", 10, 100},
		{"Medium_50files", 50, 200},
		{"Large_100files", 100, 300},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			// Setup test environment
			tmpDir := b.TempDir()
			createTestProject(b, tmpDir, size.numFiles, size.linesPerFile)

			// Create config and index
			cfg := &config.Config{
				Project: config.Project{
					Root: tmpDir,
				},
			}

			goroutineIndex := indexing.NewMasterIndex(cfg)
			ctx := context.Background()

			// Index the project once
			if err := goroutineIndex.IndexDirectory(ctx, tmpDir); err != nil {
				b.Fatalf("Failed to index: %v", err)
			}

			server, err := NewServer(goroutineIndex, cfg)
			if err != nil {
				b.Fatalf("Failed to create server: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			// Benchmark overview mode
			paramsBytes, _ := json.Marshal(CodebaseIntelligenceParams{
				Mode: "overview",
			})
			for i := 0; i < b.N; i++ {
				_, _ = server.handleCodebaseIntelligence(context.TODO(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
					Arguments: paramsBytes,
				}})
			}

			b.ReportMetric(float64(size.numFiles), "files")
			b.ReportMetric(float64(size.numFiles*size.linesPerFile), "total_lines")
		})
	}
}

// BenchmarkCodebaseIntelligenceModes benchmarks different analysis modes
func BenchmarkCodebaseIntelligenceModes(b *testing.B) {
	// Setup test environment
	tmpDir := b.TempDir()
	createTestProject(b, tmpDir, 50, 200)

	cfg := &config.Config{
		Project: config.Project{
			Root: tmpDir,
		},
	}

	goroutineIndex := indexing.NewMasterIndex(cfg)
	ctx := context.Background()

	if err := goroutineIndex.IndexDirectory(ctx, tmpDir); err != nil {
		b.Fatalf("Failed to index: %v", err)
	}

	server, err := NewServer(goroutineIndex, cfg)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	modes := []struct {
		name   string
		params CodebaseIntelligenceParams
	}{
		{
			"Overview_Tier1",
			CodebaseIntelligenceParams{
				Mode: "overview",
				Tier: intPtr(1),
			},
		},
		{
			"Overview_Tier2",
			CodebaseIntelligenceParams{
				Mode: "overview",
				Tier: intPtr(2),
			},
		},
		{
			"Detailed_Modules",
			CodebaseIntelligenceParams{
				Mode:     "detailed",
				Analysis: strPtr("modules"),
			},
		},
		{
			"Detailed_Layers",
			CodebaseIntelligenceParams{
				Mode:     "detailed",
				Analysis: strPtr("layers"),
			},
		},
		{
			"Statistics",
			CodebaseIntelligenceParams{
				Mode:    "statistics",
				Metrics: &[]string{"complexity", "coupling"},
			},
		},
	}

	for _, mode := range modes {
		b.Run(mode.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				paramsBytes, _ := json.Marshal(mode.params)
				_, _ = server.handleCodebaseIntelligence(context.TODO(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
					Arguments: paramsBytes,
				}})
			}
		})
	}
}

// BenchmarkModuleDetection benchmarks module detection performance
func BenchmarkModuleDetection(b *testing.B) {
	tmpDir := b.TempDir()

	// Create a modular project structure
	modules := []string{"auth", "api", "database", "utils", "services"}
	for _, module := range modules {
		moduleDir := filepath.Join(tmpDir, module)
		_ = os.MkdirAll(moduleDir, 0755)

		// Create files in each module
		for i := 0; i < 10; i++ {
			createGoFile(b, moduleDir, fmt.Sprintf("%s_file%d.go", module, i), module, 100)
		}
	}

	cfg := &config.Config{
		Project: config.Project{
			Root: tmpDir,
		},
	}

	goroutineIndex := indexing.NewMasterIndex(cfg)
	ctx := context.Background()

	if err := goroutineIndex.IndexDirectory(ctx, tmpDir); err != nil {
		b.Fatalf("Failed to index: %v", err)
	}

	server, err := NewServer(goroutineIndex, cfg)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Get all symbols for module detection (efficient)
		symbols, err := server.getAllSymbolsFromIndex()
		if err == nil && len(symbols) > 0 {
			totalFiles := server.goroutineIndex.GetFileCount()

			// Simulate module detection using the efficient approach
			_ = server.detectModulesByStructure(symbols, totalFiles)
		}
	}

	b.ReportMetric(float64(len(modules)), "modules")
}

// BenchmarkMemoryUsage benchmarks memory usage for different project sizes
func BenchmarkMemoryUsage(b *testing.B) {
	sizes := []int{10, 50, 100, 200}

	for _, numFiles := range sizes {
		b.Run(fmt.Sprintf("Files_%d", numFiles), func(b *testing.B) {
			tmpDir := b.TempDir()
			createTestProject(b, tmpDir, numFiles, 200)

			cfg := &config.Config{
				Project: config.Project{
					Root: tmpDir,
				},
			}

			goroutineIndex := indexing.NewMasterIndex(cfg)
			ctx := context.Background()

			if err := goroutineIndex.IndexDirectory(ctx, tmpDir); err != nil {
				b.Fatalf("Failed to index: %v", err)
			}

			server, err := NewServer(goroutineIndex, cfg)
			if err != nil {
				b.Fatalf("Failed to create server: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			// Run all analysis modes to measure total memory
			params1Bytes, _ := json.Marshal(CodebaseIntelligenceParams{Mode: "overview"})
			params2Bytes, _ := json.Marshal(CodebaseIntelligenceParams{
				Mode:     "detailed",
				Analysis: strPtr("modules"),
			})
			params3Bytes, _ := json.Marshal(CodebaseIntelligenceParams{Mode: "statistics"})
			for i := 0; i < b.N; i++ {
				// Overview
				_, _ = server.handleCodebaseIntelligence(context.TODO(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
					Arguments: params1Bytes,
				}})

				// Detailed
				_, _ = server.handleCodebaseIntelligence(context.TODO(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
					Arguments: params2Bytes,
				}})

				// Statistics
				_, _ = server.handleCodebaseIntelligence(context.TODO(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
					Arguments: params3Bytes,
				}})
			}

			// Report memory per file
			b.ReportMetric(float64(numFiles), "files_analyzed")
		})
	}
}

// BenchmarkNewQualityFunctions benchmarks the new quality analysis functions
// Uses real project indexing for accurate benchmarking
func BenchmarkNewQualityFunctions(b *testing.B) {
	// Use the existing createTestProject which works correctly
	tmpDir := b.TempDir()
	createTestProject(b, tmpDir, 100, 300) // 100 files, 300 lines each

	cfg := &config.Config{
		Project: config.Project{
			Root: tmpDir,
		},
	}

	goroutineIndex := indexing.NewMasterIndex(cfg)
	ctx := context.Background()

	if err := goroutineIndex.IndexDirectory(ctx, tmpDir); err != nil {
		b.Fatalf("Failed to index: %v", err)
	}

	server, err := NewServer(goroutineIndex, cfg)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	// Get all files for benchmarking
	allFiles := server.goroutineIndex.GetAllFiles()
	if len(allFiles) == 0 {
		// If GetAllFiles returns empty, create synthetic FileInfo for benchmarking
		b.Logf("GetAllFiles returned empty - using fallback benchmark with synthetic data")
		allFiles = createSyntheticFiles(100)
	}

	b.ReportMetric(float64(len(allFiles)), "files")
	symCount := 0
	for _, f := range allFiles {
		symCount += len(f.EnhancedSymbols)
	}
	b.ReportMetric(float64(symCount), "symbols")

	b.Run("calculateDetailedCodeSmells", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = server.calculateDetailedCodeSmells(allFiles)
		}
	})

	b.Run("identifyProblematicSymbols", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = server.identifyProblematicSymbols(allFiles)
		}
	})

	b.Run("countChildMethods", func(b *testing.B) {
		b.ReportAllocs()
		// Find a class to benchmark
		var targetFile *types.FileInfo
		var targetSym *types.EnhancedSymbol
		for _, f := range allFiles {
			for _, sym := range f.EnhancedSymbols {
				if sym.Type == types.SymbolTypeClass || sym.Type == types.SymbolTypeStruct {
					targetFile = f
					targetSym = sym
					break
				}
			}
			if targetSym != nil {
				break
			}
		}
		if targetSym == nil {
			b.Skip("No class/struct found")
		}
		for i := 0; i < b.N; i++ {
			_ = server.countChildMethods(targetFile, targetSym)
		}
	})

	b.Run("calculateSymbolRiskAndTags", func(b *testing.B) {
		b.ReportAllocs()
		if len(allFiles) == 0 || len(allFiles[0].EnhancedSymbols) == 0 {
			b.Skip("No symbols available")
		}
		sym := allFiles[0].EnhancedSymbols[0]
		for i := 0; i < b.N; i++ {
			_, _ = server.calculateSymbolRiskAndTags(sym)
		}
	})
}

// createSyntheticFiles creates synthetic FileInfo for benchmarking when real indexing fails
func createSyntheticFiles(count int) []*types.FileInfo {
	files := make([]*types.FileInfo, count)
	for i := 0; i < count; i++ {
		files[i] = &types.FileInfo{
			Path: fmt.Sprintf("file%d.go", i),
			EnhancedSymbols: []*types.EnhancedSymbol{
				{
					Symbol: types.Symbol{
						Name:    fmt.Sprintf("Function%d", i),
						Type:    types.SymbolTypeFunction,
						Line:    1,
						EndLine: 100, // Long function
					},
					Complexity:   15,                          // High complexity
					IncomingRefs: make([]types.Reference, 10), // Some refs
					OutgoingRefs: make([]types.Reference, 5),
				},
				{
					Symbol: types.Symbol{
						Name:    fmt.Sprintf("Service%d", i),
						Type:    types.SymbolTypeStruct,
						Line:    110,
						EndLine: 200,
					},
				},
			},
		}
	}
	return files
}

// Helper functions

func createTestProject(b *testing.B, dir string, numFiles, linesPerFile int) {
	b.Helper()

	// Create different types of files
	for i := 0; i < numFiles; i++ {
		var filename, content string

		switch i % 4 {
		case 0: // Go files
			filename = fmt.Sprintf("file%d.go", i)
			content = generateGoFile("main", linesPerFile)
		case 1: // JavaScript files
			filename = fmt.Sprintf("file%d.js", i)
			content = generateJSFile(linesPerFile)
		case 2: // Python files
			filename = fmt.Sprintf("file%d.py", i)
			content = generatePythonFile(linesPerFile)
		default: // Test files
			filename = fmt.Sprintf("file%d_test.go", i)
			content = generateTestFile("main", linesPerFile)
		}

		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("Failed to create file %s: %v", path, err)
		}
	}
}

func createGoFile(b *testing.B, dir, filename, pkg string, lines int) {
	b.Helper()
	content := generateGoFile(pkg, lines)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatalf("Failed to create file %s: %v", path, err)
	}
}

func generateGoFile(pkg string, lines int) string {
	content := fmt.Sprintf("package %s\n\n", pkg)
	content += "import (\n\t\"fmt\"\n\t\"strings\"\n)\n\n"

	// Generate functions
	numFuncs := lines / 20
	for i := 0; i < numFuncs; i++ {
		content += fmt.Sprintf(`
func Function%d(param string) string {
	result := strings.ToUpper(param)
	fmt.Printf("Processing: %%s\n", result)
	if len(result) > 10 {
		return result[:10]
	}
	return result
}

`, i)
	}

	// Generate a struct
	content += fmt.Sprintf(`
type Service%s struct {
	Name    string
	Version string
	Config  map[string]interface{}
}

func (s *Service%s) Process(data []byte) error {
	// Process data
	return nil
}
`, pkg, pkg)

	return content
}

func generateJSFile(lines int) string {
	content := "// JavaScript file\n\n"

	numFuncs := lines / 15
	for i := 0; i < numFuncs; i++ {
		content += fmt.Sprintf(`
function processData%d(input) {
	const result = input.toUpperCase();
	console.log('Processing:', result);
	return result.substring(0, 10);
}

class Service%d {
	constructor(name) {
		this.name = name;
		this.data = [];
	}

	process(item) {
		this.data.push(item);
		return this.data.length;
	}
}

`, i, i)
	}

	return content
}

func generatePythonFile(lines int) string {
	content := "# Python file\n\n"

	numFuncs := lines / 15
	for i := 0; i < numFuncs; i++ {
		content += fmt.Sprintf(`
def process_data_%d(input_str):
    """Process input data"""
    result = input_str.upper()
    print(f"Processing: {result}")
    return result[:10] if len(result) > 10 else result

class Service%d:
    """Service class"""

    def __init__(self, name):
        self.name = name
        self.data = []

    def process(self, item):
        self.data.append(item)
        return len(self.data)

`, i, i)
	}

	return content
}

func generateTestFile(pkg string, lines int) string {
	content := fmt.Sprintf("package %s\n\n", pkg)
	content += "import \"testing\"\n\n"

	numTests := lines / 10
	for i := 0; i < numTests; i++ {
		content += fmt.Sprintf(`
func TestFunction%d(t *testing.T) {
	result := Function%d("test")
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

`, i, i)
	}

	return content
}

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}
