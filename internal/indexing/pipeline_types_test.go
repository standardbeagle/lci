package indexing

import (
	"os"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/types"
	"github.com/standardbeagle/lci/testhelpers"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestPipelineConstants(t *testing.T) {
	t.Run("task channel timeout", func(t *testing.T) {
		if taskChannelTimeout != 5*time.Second {
			t.Errorf("Expected taskChannelTimeout = 5s, got %v", taskChannelTimeout)
		}
	})

	t.Run("task channel buffer base multiplier", func(t *testing.T) {
		if taskChannelBufferBaseMultiplier != 8 {
			t.Errorf("Expected taskChannelBufferBaseMultiplier = 8, got %d", taskChannelBufferBaseMultiplier)
		}
	})

	t.Run("result channel buffer base multiplier", func(t *testing.T) {
		if resultChannelBufferBaseMultiplier != 16 {
			t.Errorf("Expected resultChannelBufferBaseMultiplier = 16, got %d", resultChannelBufferBaseMultiplier)
		}
	})

	t.Run("estimated scanning progress", func(t *testing.T) {
		if estimatedScanningProgress != 50.0 {
			t.Errorf("Expected estimatedScanningProgress = 50.0, got %f", estimatedScanningProgress)
		}
	})
}

func TestFileTask(t *testing.T) {
	t.Run("create file task with all fields", func(t *testing.T) {
		// Create a temporary file for testing
		tmpFile, err := os.CreateTemp("", "test_file_task_*.go")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		fileInfo, err := tmpFile.Stat()
		if err != nil {
			t.Fatalf("Failed to get file info: %v", err)
		}

		task := FileTask{
			Path:     tmpFile.Name(),
			Info:     fileInfo,
			Priority: 10,
		}

		if task.Path != tmpFile.Name() {
			t.Errorf("Expected Path = '%s', got '%s'", tmpFile.Name(), task.Path)
		}

		if task.Info.Size() != fileInfo.Size() {
			t.Errorf("Expected Info.Size() = %d, got %d", fileInfo.Size(), task.Info.Size())
		}

		if task.Priority != 10 {
			t.Errorf("Expected Priority = 10, got %d", task.Priority)
		}

		if task.Info.IsDir() != false {
			t.Errorf("Expected file to not be a directory")
		}
	})

	t.Run("create file task with minimal fields", func(t *testing.T) {
		task := FileTask{
			Path:     "/test/path/file.go",
			Priority: 0,
		}

		if task.Path != "/test/path/file.go" {
			t.Errorf("Expected Path = '/test/path/file.go', got '%s'", task.Path)
		}

		if task.Priority != 0 {
			t.Errorf("Expected Priority = 0, got %d", task.Priority)
		}

		// Info should be zero value
		if task.Info != nil {
			t.Errorf("Expected Info = nil, got %v", task.Info)
		}
	})

	t.Run("file task priority ordering", func(t *testing.T) {
		task1 := FileTask{Path: "low.go", Priority: 1}
		task2 := FileTask{Path: "high.go", Priority: 10}
		task3 := FileTask{Path: "medium.go", Priority: 5}

		// Higher priority should come first
		if task2.Priority <= task3.Priority {
			t.Errorf("High priority task should have higher priority than medium")
		}

		if task3.Priority <= task1.Priority {
			t.Errorf("Medium priority task should have higher priority than low")
		}
	})
}

func TestProcessedFile(t *testing.T) {
	t.Run("create processed file with all fields", func(t *testing.T) {
		symbols := []types.Symbol{
			{Name: "TestFunction", Type: types.SymbolTypeFunction, FileID: types.FileID(1)},
			{Name: "TestVariable", Type: types.SymbolTypeVariable, FileID: types.FileID(1)},
		}

		references := []types.Reference{
			{SourceSymbol: types.SymbolID(1), TargetSymbol: types.SymbolID(1), FileID: types.FileID(1)},
		}

		scopes := []types.ScopeInfo{
			{Type: types.ScopeTypeFunction, Name: "test", StartLine: 1, EndLine: 10},
		}

		content := []byte("package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}")
		lineOffsets := []int{0, 13, 28, 59}
		language := "go"

		processed := ProcessedFile{
			Path:        "/test/path/file.go",
			FileID:      types.FileID(42),
			Symbols:     symbols,
			References:  references,
			Scopes:      scopes,
			Content:     content,
			LineOffsets: lineOffsets,
			Language:    language,
			Stage:       "indexing",
			Duration:    5 * time.Millisecond,
		}

		if processed.Path != "/test/path/file.go" {
			t.Errorf("Expected Path = '/test/path/file.go', got '%s'", processed.Path)
		}

		if processed.FileID != types.FileID(42) {
			t.Errorf("Expected FileID = 42, got %d", processed.FileID)
		}

		if len(processed.Symbols) != 2 {
			t.Errorf("Expected 2 symbols, got %d", len(processed.Symbols))
		}

		if processed.Symbols[0].Name != "TestFunction" {
			t.Errorf("Expected first symbol name = 'TestFunction', got '%s'", processed.Symbols[0].Name)
		}

		if len(processed.References) != 1 {
			t.Errorf("Expected 1 reference, got %d", len(processed.References))
		}

		if len(processed.Scopes) != 1 {
			t.Errorf("Expected 1 scope, got %d", len(processed.Scopes))
		}

		if string(processed.Content) != "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}" {
			t.Errorf("Expected content mismatch")
		}

		if len(processed.LineOffsets) != 4 {
			t.Errorf("Expected 4 line offsets, got %d", len(processed.LineOffsets))
		}

		if processed.Language != "go" {
			t.Errorf("Expected Language = 'go', got '%s'", processed.Language)
		}

		if processed.Stage != "indexing" {
			t.Errorf("Expected Stage = 'indexing', got '%s'", processed.Stage)
		}

		if processed.Duration != 5*time.Millisecond {
			t.Errorf("Expected Duration = 5ms, got %v", processed.Duration)
		}

		if processed.Error != nil {
			t.Errorf("Expected Error = nil, got %v", processed.Error)
		}

		if processed.AST != nil {
			t.Errorf("Expected AST = nil, got %v", processed.AST)
		}
	})

	t.Run("create processed file with error", func(t *testing.T) {
		testError := &os.PathError{Op: "open", Path: "/nonexistent", Err: os.ErrNotExist}

		processed := ProcessedFile{
			Path:     "/nonexistent/file.go",
			FileID:   types.FileID(0),
			Symbols:  []types.Symbol{},
			Stage:    "parsing",
			Duration: 1 * time.Millisecond,
			Error:    testError,
		}

		if processed.Error == nil {
			t.Error("Expected Error to be set")
		}

		if processed.Error.Error() != testError.Error() {
			t.Errorf("Expected error message = '%s', got '%s'", testError.Error(), processed.Error.Error())
		}

		if processed.Stage != "parsing" {
			t.Errorf("Expected Stage = 'parsing', got '%s'", processed.Stage)
		}
	})

	t.Run("create processed file with minimal fields", func(t *testing.T) {
		processed := ProcessedFile{
			Path: "/minimal/file.go",
		}

		if processed.Path != "/minimal/file.go" {
			t.Errorf("Expected Path = '/minimal/file.go', got '%s'", processed.Path)
		}

		if processed.FileID != types.FileID(0) {
			t.Errorf("Expected FileID = 0, got %d", processed.FileID)
		}

		if processed.Symbols != nil {
			t.Errorf("Expected Symbols = nil, got %v", processed.Symbols)
		}

		if processed.References != nil {
			t.Errorf("Expected References = nil, got %v", processed.References)
		}

		if processed.Content != nil {
			t.Errorf("Expected Content = nil, got %v", processed.Content)
		}

		if processed.Language != "" {
			t.Errorf("Expected Language = '', got '%s'", processed.Language)
		}

		if processed.Stage != "" {
			t.Errorf("Expected Stage = '', got '%s'", processed.Stage)
		}

		if processed.Duration != 0 {
			t.Errorf("Expected Duration = 0, got %v", processed.Duration)
		}
	})

	t.Run("processed file with AST", func(t *testing.T) {
		// Create a mock AST (in real usage this would come from tree-sitter)
		mockAST := &tree_sitter.Tree{}

		processed := ProcessedFile{
			Path:     "/test/ast_file.go",
			FileID:   types.FileID(1),
			AST:      mockAST,
			Language: "go",
			Stage:    "parsing",
		}

		if processed.AST != mockAST {
			t.Error("AST not set correctly")
		}

		if processed.Language != "go" {
			t.Errorf("Expected Language = 'go', got '%s'", processed.Language)
		}
	})

	t.Run("processed file content metrics", func(t *testing.T) {
		content := []byte("line1\nline2\nline3\nline4\nline5")
		processed := ProcessedFile{
			Path:    "/test/metrics.go",
			Content: content,
		}

		if len(processed.Content) != len(content) {
			t.Errorf("Expected content length = %d, got %d", len(content), len(processed.Content))
		}

		if processed.Content[0] != byte('l') {
			t.Errorf("Expected first byte = 'l', got '%c'", processed.Content[0])
		}

		if processed.Content[len(processed.Content)-1] != byte('5') {
			t.Errorf("Expected last byte = '5', got '%c'", processed.Content[len(processed.Content)-1])
		}
	})
}

func TestFileScanner(t *testing.T) {
	t.Run("create file scanner with config", func(t *testing.T) {
		cfg := testhelpers.NewTestConfigBuilder(t.TempDir()).Build()

		scanner := FileScanner{
			config:         cfg,
			bufferSize:     1024,
			binaryDetector: NewBinaryDetector(),
		}

		if scanner.config != cfg {
			t.Error("config not set correctly")
		}

		if scanner.bufferSize != 1024 {
			t.Errorf("Expected bufferSize = 1024, got %d", scanner.bufferSize)
		}

		if scanner.gitignoreParser != nil {
			t.Error("gitignoreParser should be nil by default")
		}

		if scanner.binaryDetector == nil {
			t.Error("binaryDetector should be initialized")
		}
	})

	t.Run("create file scanner with minimal fields", func(t *testing.T) {
		scanner := FileScanner{}

		if scanner.config != nil {
			t.Error("Expected config = nil")
		}

		if scanner.bufferSize != 0 {
			t.Errorf("Expected bufferSize = 0, got %d", scanner.bufferSize)
		}

		if scanner.gitignoreParser != nil {
			t.Error("Expected gitignoreParser = nil")
		}
	})

	t.Run("file scanner with gitignore parser", func(t *testing.T) {
		// Create a temporary gitignore file
		tmpDir, err := os.MkdirTemp("", "test_gitignore")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		gitignorePath := tmpDir + "/.gitignore"
		gitignoreContent := "*.log\nbuild/\ntemp/"
		err = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write gitignore file: %v", err)
		}

		parser := config.NewGitignoreParser()

		scanner := FileScanner{
			gitignoreParser: parser,
			binaryDetector:  NewBinaryDetector(),
		}

		if scanner.gitignoreParser == nil {
			t.Error("gitignoreParser should be set")
		}

		if scanner.gitignoreParser != parser {
			t.Error("gitignoreParser not set correctly")
		}

		if scanner.binaryDetector == nil {
			t.Error("binaryDetector should be initialized")
		}
	})
}

func TestProcessedFileValidation(t *testing.T) {
	t.Run("validate processed file consistency", func(t *testing.T) {
		processed := ProcessedFile{
			Path:       "/test/consistent.go",
			FileID:     types.FileID(1),
			Symbols:    []types.Symbol{{Name: "func1", Type: types.SymbolTypeFunction, FileID: types.FileID(1)}},
			References: []types.Reference{{SourceSymbol: types.SymbolID(1), TargetSymbol: types.SymbolID(1), FileID: types.FileID(1)}},
			Stage:      "indexing",
		}

		// Basic consistency checks
		if processed.Path == "" {
			t.Error("Path should not be empty")
		}

		if processed.Stage == "" {
			t.Error("Stage should not be empty")
		}

		// If there are symbols, they should have valid names and types
		for _, symbol := range processed.Symbols {
			if symbol.Name == "" {
				t.Errorf("Symbol should have non-empty name")
			}
			if symbol.FileID == types.FileID(0) && symbol.Name != "" {
				t.Errorf("Symbol '%s' should have non-zero FileID", symbol.Name)
			}
		}

		// If there are references, they should have valid IDs
		for _, ref := range processed.References {
			if ref.SourceSymbol == types.SymbolID(0) {
				t.Error("Reference should have non-zero SourceSymbol")
			}
			if ref.FileID == types.FileID(0) {
				t.Error("Reference should have non-zero FileID")
			}
		}
	})

	t.Run("validate processed file stages", func(t *testing.T) {
		validStages := []string{"scanning", "parsing", "indexing", "completed"}

		for _, stage := range validStages {
			processed := ProcessedFile{
				Path:  "/test/stage.go",
				Stage: stage,
			}

			if processed.Stage != stage {
				t.Errorf("Stage '%s' not preserved", stage)
			}
		}
	})
}

// Benchmark tests
func BenchmarkFileTaskCreation(b *testing.B) {
	fileInfo, _ := os.Stat("/dev/null") // Use any file for benchmarking

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FileTask{
			Path:     "/test/path/file.go",
			Info:     fileInfo,
			Priority: i % 10,
		}
	}
}

func BenchmarkProcessedFileCreation(b *testing.B) {
	symbols := []types.Symbol{
		{Name: "TestFunction", Type: types.SymbolTypeFunction, FileID: types.FileID(1)},
		{Name: "TestVariable", Type: types.SymbolTypeVariable, FileID: types.FileID(1)},
	}

	content := make([]byte, 1024) // 1KB content

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ProcessedFile{
			Path:     "/test/path/file.go",
			FileID:   types.FileID(i),
			Symbols:  symbols,
			Content:  content,
			Language: "go",
			Stage:    "indexing",
			Duration: time.Microsecond,
		}
	}
}

func BenchmarkFileScannerCreation(b *testing.B) {
	cfg := testhelpers.NewTestConfigBuilder(b.TempDir()).Build()
	parser := config.NewGitignoreParser()
	detector := NewBinaryDetector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FileScanner{
			config:          cfg,
			bufferSize:      1024,
			gitignoreParser: parser,
			binaryDetector:  detector,
		}
	}
}
