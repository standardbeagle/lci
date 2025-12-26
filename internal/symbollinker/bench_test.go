package symbollinker

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// BenchmarkSymbolExtraction benchmarks symbol extraction across different languages
func BenchmarkSymbolExtraction(b *testing.B) {
	tempDir := b.TempDir()
	engine := NewSymbolLinkerEngine(tempDir)

	// Test data for different languages
	testFiles := map[string]string{
		"complex.go": generateComplexGoCode(100), // 100 symbols
		"complex.js": generateComplexJSCode(100), // 100 symbols
		"complex.ts": generateComplexTSCode(100), // 100 symbols
	}

	// Pre-create files
	for filename, content := range testFiles {
		fullPath := filepath.Join(tempDir, filename)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.Run("Go", func(b *testing.B) {
		content := []byte(testFiles["complex.go"])
		goPath := filepath.Join(tempDir, "complex.go")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := engine.IndexFile(goPath, content)
			if err != nil {
				b.Fatal(err)
			}
		}

		b.ReportMetric(float64(len(content)), "bytes")
		b.ReportMetric(100, "symbols")
	})

	b.Run("JavaScript", func(b *testing.B) {
		content := []byte(testFiles["complex.js"])
		jsPath := filepath.Join(tempDir, "complex.js")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := engine.IndexFile(jsPath, content)
			if err != nil {
				b.Fatal(err)
			}
		}

		b.ReportMetric(float64(len(content)), "bytes")
		b.ReportMetric(100, "symbols")
	})

	b.Run("TypeScript", func(b *testing.B) {
		content := []byte(testFiles["complex.ts"])
		tsPath := filepath.Join(tempDir, "complex.ts")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := engine.IndexFile(tsPath, content)
			if err != nil {
				b.Fatal(err)
			}
		}

		b.ReportMetric(float64(len(content)), "bytes")
		b.ReportMetric(100, "symbols")
	})
}

// BenchmarkSymbolLinking benchmarks the cross-file symbol linking process
func BenchmarkSymbolLinking(b *testing.B) {
	sizes := []struct {
		name  string
		files int
	}{
		{"Small", 10},
		{"Medium", 50},
		{"Large", 100},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			tempDir := b.TempDir()
			engine := NewSymbolLinkerEngine(tempDir)

			// Create interconnected files
			files := generateInterconnectedFiles(size.files)

			// Index all files once
			for filename, content := range files {
				fullPath := filepath.Join(tempDir, filename)
				err := os.WriteFile(fullPath, []byte(content), 0644)
				if err != nil {
					b.Fatal(err)
				}

				err = engine.IndexFile(fullPath, []byte(content))
				if err != nil {
					b.Fatal(err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := engine.LinkSymbols()
				if err != nil {
					b.Fatal(err)
				}
			}

			stats := engine.Stats()
			b.ReportMetric(float64(stats["files"]), "files")
			b.ReportMetric(float64(stats["symbols"]), "symbols")
			b.ReportMetric(float64(stats["import_links"]), "links")
		})
	}
}

// BenchmarkIncrementalUpdates benchmarks incremental update performance
func BenchmarkIncrementalUpdates(b *testing.B) {
	tempDir := b.TempDir()
	engine := NewIncrementalEngine(tempDir)

	// Create initial project with dependencies
	files := generateInterconnectedFiles(50)

	// Index all files initially
	for filename, content := range files {
		fullPath := filepath.Join(tempDir, filename)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}

		err = engine.IndexFile(fullPath, []byte(content))
		if err != nil {
			b.Fatal(err)
		}
	}

	err := engine.LinkSymbols()
	if err != nil {
		b.Fatal(err)
	}

	// Prepare update content
	updateContent := generateModifiedContent("service_0.go")
	updatePath := filepath.Join(tempDir, "service_0.go")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.UpdateFile(updatePath, []byte(updateContent))
		if err != nil {
			b.Fatal(err)
		}
	}

	incStats := engine.IncrementalStats()
	if trackedFiles, ok := incStats["tracked_files"].(int); ok {
		b.ReportMetric(float64(trackedFiles), "tracked_files")
	}
	if depEdges, ok := incStats["dependency_edges"].(int); ok {
		b.ReportMetric(float64(depEdges), "dep_edges")
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	tempDir := b.TempDir()
	engine := NewSymbolLinkerEngine(tempDir)

	// Create large project
	files := generateInterconnectedFiles(200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Index all files
		for filename, content := range files {
			fullPath := filepath.Join(tempDir, filename)
			err := os.WriteFile(fullPath, []byte(content), 0644)
			if err != nil {
				b.Fatal(err)
			}

			err = engine.IndexFile(fullPath, []byte(content))
			if err != nil {
				b.Fatal(err)
			}
		}

		err := engine.LinkSymbols()
		if err != nil {
			b.Fatal(err)
		}

		if i == 0 {
			runtime.GC()
			runtime.ReadMemStats(&m2)

			b.ReportMetric(float64(m2.Alloc-m1.Alloc)/1024/1024, "MB_alloc")
			b.ReportMetric(float64(m2.Sys-m1.Sys)/1024/1024, "MB_sys")
		}
	}
}

// BenchmarkConcurrentAccess benchmarks concurrent symbol operations
func BenchmarkConcurrentAccess(b *testing.B) {
	tempDir := b.TempDir()
	engine := NewSymbolLinkerEngine(tempDir)

	// Set up test data
	files := generateInterconnectedFiles(20)
	for filename, content := range files {
		fullPath := filepath.Join(tempDir, filename)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}

		err = engine.IndexFile(fullPath, []byte(content))
		if err != nil {
			b.Fatal(err)
		}
	}

	err := engine.LinkSymbols()
	if err != nil {
		b.Fatal(err)
	}

	// Get some file IDs for concurrent access
	fileIDs := make([]types.FileID, 0, 5)
	i := 0
	for filename := range files {
		if i >= 5 {
			break
		}
		fullPath := filepath.Join(tempDir, filename)
		fileIDs = append(fileIDs, engine.GetOrCreateFileID(fullPath))
		i++
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Concurrent read operations
			for _, fileID := range fileIDs {
				symbols, err := engine.GetSymbolsInFile(fileID)
				if err != nil {
					b.Error(err)
					return
				}

				if len(symbols) == 0 {
					continue
				}

				// Test getting imports
				imports, err := engine.GetFileImports(fileID)
				if err != nil {
					b.Error(err)
					return
				}
				_ = imports // Use the result
			}
		}
	})
}

// Helper functions for generating test data

func generateComplexGoCode(numSymbols int) string {
	var builder strings.Builder

	builder.WriteString("package main\n\nimport (\n")
	builder.WriteString("\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n\n")

	// Generate structs, functions, variables, constants
	for i := 0; i < numSymbols/4; i++ {
		// Struct
		builder.WriteString(fmt.Sprintf(`type Service%d struct {
	ID   int
	Name string
	Data map[string]interface{}
}

`, i))

		// Constructor function
		builder.WriteString(fmt.Sprintf(`func NewService%d(id int, name string) *Service%d {
	return &Service%d{
		ID:   id,
		Name: name,
		Data: make(map[string]interface{}),
	}
}

`, i, i, i))

		// Method
		builder.WriteString(fmt.Sprintf(`func (s *Service%d) Process(input string) error {
	s.Data["processed"] = strings.ToUpper(input)
	fmt.Printf("Service %%d processed: %%s\\n", s.ID, input)
	return nil
}

`, i))

		// Constant
		builder.WriteString(fmt.Sprintf("const SERVICE_%d_VERSION = \"1.%d.0\"\n\n", i, i))
	}

	return builder.String()
}

func generateComplexJSCode(numSymbols int) string {
	var builder strings.Builder

	// Generate classes, functions, constants
	for i := 0; i < numSymbols/5; i++ {
		// Class
		builder.WriteString(fmt.Sprintf(`export class DataProcessor%d {
	constructor(config) {
		this.config = config;
		this.id = %d;
		this.processors = new Map();
	}

	async process(data) {
		const result = await this.transform(data);
		return this.validate(result);
	}

	transform(data) {
		return {
			...data,
			processed: true,
			timestamp: new Date(),
			processorId: this.id
		};
	}

	validate(data) {
		return data && typeof data === 'object';
	}

	static createInstance(config) {
		return new DataProcessor%d(config);
	}
}

`, i, i, i))

		// Function
		builder.WriteString(fmt.Sprintf(`export function processData%d(input, options = {}) {
	const processor = new DataProcessor%d(options);
	return processor.process(input);
}

`, i, i))

		// Constant
		builder.WriteString(fmt.Sprintf(`export const PROCESSOR_%d_CONFIG = {
	version: '1.%d.0',
	enabled: true,
	maxRetries: 3
};

`, i, i))
	}

	return builder.String()
}

func generateComplexTSCode(numSymbols int) string {
	var builder strings.Builder

	// Generate interfaces, types, classes
	for i := 0; i < numSymbols/6; i++ {
		// Interface
		builder.WriteString(fmt.Sprintf(`export interface DataModel%d {
	id: number;
	name: string;
	metadata: Record<string, unknown>;
	createdAt: Date;
	updatedAt?: Date;
}

`, i))

		// Type alias
		builder.WriteString(fmt.Sprintf(`export type ProcessorResult%d<T = any> = {
	success: boolean;
	data?: T;
	error?: string;
	processorId: number;
};

`, i))

		// Class
		builder.WriteString(fmt.Sprintf(`export class ModelProcessor%d<T extends DataModel%d> {
	private readonly id: number = %d;
	private cache: Map<number, T> = new Map();

	constructor(private config: ProcessorConfig) {}

	async process(model: T): Promise<ProcessorResult%d<T>> {
		try {
			const processed = await this.transform(model);
			this.cache.set(model.id, processed);
			return { success: true, data: processed, processorId: this.id };
		} catch (error) {
			return { 
				success: false, 
				error: error instanceof Error ? error.message : 'Unknown error',
				processorId: this.id 
			};
		}
	}

	private async transform(model: T): Promise<T> {
		return {
			...model,
			updatedAt: new Date(),
			metadata: { ...model.metadata, processed: true }
		};
	}

	getFromCache(id: number): T | undefined {
		return this.cache.get(id);
	}
}

`, i, i, i, i))

		// Enum
		builder.WriteString(fmt.Sprintf(`export enum ProcessorStatus%d {
	IDLE = 'idle',
	PROCESSING = 'processing',
	COMPLETED = 'completed',
	ERROR = 'error'
}

`, i))
	}

	// Common types
	builder.WriteString(`interface ProcessorConfig {
	maxConcurrency: number;
	timeout: number;
	retries: number;
}
`)

	return builder.String()
}

func generateInterconnectedFiles(numFiles int) map[string]string {
	files := make(map[string]string)

	for i := 0; i < numFiles; i++ {
		if i%3 == 0 {
			// Go file that imports from other Go files
			var imports strings.Builder
			imports.WriteString("package main\n\nimport (\n\t\"fmt\"\n")

			// Import from next file if it exists
			if i+1 < numFiles {
				imports.WriteString(fmt.Sprintf("\t\"./service_%d\"\n", i+1))
			}
			imports.WriteString(")\n\n")

			content := imports.String() + fmt.Sprintf(`type Manager%d struct {
	services []interface{}
	config   map[string]string
}

func NewManager%d() *Manager%d {
	return &Manager%d{
		services: make([]interface{}, 0),
		config:   make(map[string]string),
	}
}

func (m *Manager%d) AddService(service interface{}) {
	m.services = append(m.services, service)
}

func (m *Manager%d) Process(data string) error {
	fmt.Printf("Manager %d processing: %%s\\n", data)
	return nil
}
`, i, i, i, i, i, i, i)

			files[fmt.Sprintf("manager_%d.go", i)] = content

		} else if i%3 == 1 {
			// JavaScript file that imports from other JS files
			var imports strings.Builder

			// Import from previous file if it exists
			if i > 0 {
				imports.WriteString(fmt.Sprintf("import { Manager%d } from './manager_%d.js';\n", i-1, i-1))
			}

			content := imports.String() + fmt.Sprintf(`export class Service%d {
	constructor(config = {}) {
		this.id = %d;
		this.config = config;
		this.active = true;
	}

	async execute(data) {
		if (!this.active) {
			throw new Error('Service is not active');
		}
		
		const result = await this.processData(data);
		return {
			serviceId: this.id,
			result,
			timestamp: new Date()
		};
	}

	async processData(data) {
		// Simulate async processing
		return new Promise(resolve => {
			setTimeout(() => {
				resolve({ processed: true, data });
			}, 1);
		});
	}

	shutdown() {
		this.active = false;
	}
}

export const SERVICE_%d_CONFIG = {
	version: '1.0.%d',
	timeout: 5000,
	retries: 3
};
`, i, i, i, i)

			files[fmt.Sprintf("service_%d.js", i)] = content

		} else {
			// TypeScript file that imports from both Go and JS concepts
			var imports strings.Builder

			// Import from JS service if it exists
			if i > 1 {
				imports.WriteString(fmt.Sprintf("import { Service%d, SERVICE_%d_CONFIG } from './service_%d.js';\n", i-1, i-1, i-1))
			}

			content := imports.String() + fmt.Sprintf(`export interface Config%d {
	timeout: number;
	retries: number;
	debug: boolean;
	serviceId: number;
}

export class Controller%d {
	private config: Config%d;
	private services: Map<number, any> = new Map();

	constructor(config: Config%d) {
		this.config = config;
	}

	async initialize(): Promise<void> {
		// Initialize services
		for (let i = 0; i < 3; i++) {
			const service = { id: i, active: true };
			this.services.set(i, service);
		}
	}

	async executeAll(data: unknown): Promise<Array<{ serviceId: number; result: any }>> {
		const results: Array<{ serviceId: number; result: any }> = [];
		
		for (const [id, service] of this.services) {
			try {
				const result = await this.executeService(id, data);
				results.push({ serviceId: id, result });
			} catch (error) {
				console.error(`+"`Service ${id} failed: ${error}`"+`);
			}
		}
		
		return results;
	}

	private async executeService(id: number, data: unknown): Promise<any> {
		return { processed: true, serviceId: id, data };
	}
}

export const CONTROLLER_%d_DEFAULTS: Config%d = {
	timeout: 10000,
	retries: 5,
	debug: false,
	serviceId: %d
};
`, i, i, i, i, i, i, i)

			files[fmt.Sprintf("controller_%d.ts", i)] = content
		}
	}

	return files
}

func generateModifiedContent(filename string) string {
	return `package main

import (
	"fmt"
	"time"
)

type UpdatedManager struct {
	services []interface{}
	config   map[string]string
	lastUpdate time.Time
}

func NewUpdatedManager() *UpdatedManager {
	return &UpdatedManager{
		services: make([]interface{}, 0),
		config:   make(map[string]string),
		lastUpdate: time.Now(),
	}
}

func (m *UpdatedManager) AddService(service interface{}) {
	m.services = append(m.services, service)
	m.lastUpdate = time.Now()
}

func (m *UpdatedManager) Process(data string) error {
	fmt.Printf("Updated manager processing: %s\n", data)
	m.lastUpdate = time.Now()
	return nil
}

func (m *UpdatedManager) GetLastUpdate() time.Time {
	return m.lastUpdate
}
`
}
