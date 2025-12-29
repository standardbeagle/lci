# FileService Abstraction Layer

## Purpose and Critical Importance

The FileService abstraction is a critical architectural boundary in the Lightning Code Index (LCI) that separates filesystem operations from core indexing and search logic. This abstraction serves several essential purposes:

1. **Testability**: By abstracting filesystem operations through the FileService interface, we enable comprehensive testing without requiring actual files on disk. Tests can use mock FileService implementations to simulate various filesystem conditions.

2. **Performance Optimization**: The FileService layer provides centralized file content caching, line offset caching, and efficient file metadata management. This prevents redundant file I/O operations and significantly improves indexing and search performance.

3. **Architectural Consistency**: The FileService abstraction creates a clear separation of concerns. Core components (indexing, search, analysis) focus on their domain logic without mixing in filesystem concerns.

4. **Future Extensibility**: The abstraction enables future support for non-filesystem sources (remote files, in-memory virtual filesystems, network-based code repositories) without requiring changes to core components.

5. **Error Handling**: Centralized error handling for file operations ensures consistent behavior across the codebase and simplifies error recovery strategies.

## Strict Compliance Rules

The following rules MUST be followed by all production code in critical components:

### Forbidden Direct Filesystem Operations

Production code in these critical packages MUST NOT use direct filesystem operations:
- `internal/indexing/` - Indexing logic
- `internal/core/` - Core data structures and algorithms (except FileService implementation)
- `internal/search/` - Search engine logic
- `internal/mcp/` - MCP server logic (except diagnostic infrastructure)

### Forbidden Patterns

The following patterns are **strictly forbidden** in critical components:

```go
// ❌ FORBIDDEN: Direct file reading
content, err := os.ReadFile(path)

// ❌ FORBIDDEN: Direct file stat
info, err := os.Stat(path)

// ❌ FORBIDDEN: Direct file writing
err := os.WriteFile(path, data, 0644)

// ❌ FORBIDDEN: Direct directory creation
err := os.MkdirAll(dirPath, 0755)

// ❌ FORBIDDEN: Direct file opening
file, err := os.Open(path)

// ❌ FORBIDDEN: Direct directory walking
filepath.Walk(root, walkFunc)

// ❌ FORBIDDEN: ioutil package (deprecated)
content, err := ioutil.ReadFile(path)
```

### Allowed Exceptions

The following components are **allowed** to use direct filesystem operations:

1. **FileService implementation itself** (`file_service.go`, `file_content_store.go`, `file_loader.go`)
2. **CLI layer** (`cmd/` directory) - for configuration, profiling, and user I/O
3. **Configuration loading** (`kdl_config.go`) - legitimate direct filesystem use
4. **Persistence layer** (`persistence.go`) - for index persistence
5. **Pipeline and file scanning** (`pipeline.go`, `pipeline_scanner.go`, `watcher.go`) - file system monitoring
6. **Test infrastructure** (`_test.go` files, `internal/testing/`, `testhelpers/`)
7. **MCP diagnostics** (`diagnostics.go`) - must work before FileService initialization
8. **Context manifest tool** (`context_manifest_tool.go`) - cross-session portable metadata

## Valid Usage Patterns

### Reading File Contents

```go
// ✅ CORRECT: Use FileService to read file contents
content, ok := fileService.GetFileContent(fileID)
if !ok {
    return fmt.Errorf("failed to get file content")
}

// Use content for processing
processContent(content)
```

### Getting File Metadata

```go
// ✅ CORRECT: Use FileService to get file info
fileInfo, ok := fileService.GetFileInfo(fileID)
if !ok {
    return fmt.Errorf("file not found")
}

// Access metadata through FileInfo
path := fileInfo.Path
lineCount := len(fileInfo.LineOffsets)
```

### Getting File Paths

```go
// ✅ CORRECT: Use FileService lightweight accessor
path := fileService.GetFilePath(fileID)

// For performance-critical code, use lightweight accessors
offsets, ok := fileService.GetFileLineOffsets(fileID)
```

### Scanning Directories

```go
// ✅ CORRECT: Use scanner components that integrate with FileService
scanner := NewFileScanner(config, maxFileCount)
fileCount, totalBytes, err := scanner.CountFiles(ctx, projectRoot)
```

## Component Responsibilities

### FileService Interface

The FileService interface provides the following core operations:

1. **File Content Access**
   - `GetFileContent(fileID) ([]byte, bool)` - Retrieve cached file contents
   - `GetFilePath(fileID) string` - Get file path for a file ID

2. **File Metadata Access**
   - `GetFileInfo(fileID) (*types.FileInfo, bool)` - Get complete file information
   - `GetFileLineOffsets(fileID) ([]uint32, bool)` - Get line offset data
   - `GetFileLineCount(fileID) int` - Get line count for a file

3. **Performance Optimization**
   - Internal content caching to avoid redundant I/O
   - Line offset caching for fast line number lookups
   - Efficient batch operations for multi-file processing

### Core Components Using FileService

1. **Indexing Layer** (`internal/indexing/`)
   - Uses FileService to read source files during indexing
   - Accesses file metadata for symbol resolution
   - Never directly touches filesystem

2. **Search Engine** (`internal/search/`)
   - Uses FileService to access file contents for searching
   - Retrieves line offsets for result formatting
   - Relies on FileService caching for performance

3. **MCP Server** (`internal/mcp/`)
   - Uses FileService to serve code context to AI assistants
   - Provides file contents without direct filesystem access
   - Exception: diagnostic logging before FileService initialization

### FileService Implementation

The FileService implementation in `internal/core/` provides:

1. **Content Store** (`file_content_store.go`)
   - Manages in-memory file content cache
   - Handles cache eviction policies
   - Provides thread-safe concurrent access

2. **File Loader** (`file_loader.go`)
   - Loads files from disk into the content store
   - Handles binary file detection
   - Manages file size limits

3. **Integration Points**
   - Pipeline integration for file discovery
   - Indexer integration for content access
   - Search engine integration for result retrieval

## Migration Strategy

### For Existing Code with Violations

If you encounter code that violates the FileService abstraction:

1. **Identify the filesystem operation**
   ```go
   // Current violation
   content, err := os.ReadFile(filePath)
   ```

2. **Determine the file identifier**
   - If you have a `types.FileID`, use it directly
   - If you have a path, convert to FileID through the indexer

3. **Replace with FileService call**
   ```go
   // Migrated to FileService
   content, ok := fileService.GetFileContent(fileID)
   if !ok {
       return fmt.Errorf("failed to get file content for %v", fileID)
   }
   ```

4. **Test the migration**
   - Ensure unit tests pass
   - Run FileService compliance checker
   - Verify performance is maintained or improved

### For New Code

When writing new code:

1. **Design with FileService from the start**
   - Accept `FileService` interface as a dependency
   - Use FileService methods for all file operations
   - Never import `os` or `ioutil` for file I/O

2. **Use dependency injection**
   ```go
   type MyComponent struct {
       fileService FileService
   }

   func NewMyComponent(fs FileService) *MyComponent {
       return &MyComponent{fileService: fs}
   }
   ```

3. **Write testable code**
   - Tests can provide mock FileService
   - No filesystem dependency in tests
   - Fast, reliable test execution

### Compliance Checking

The codebase includes automated compliance checking:

```bash
# Run FileService compliance tests
go test -v ./internal/testing/ -run TestFileServiceCompliance
```

The compliance checker scans for forbidden patterns and reports violations with file and line number information.

## Benefits and Trade-offs

### Benefits

1. **Faster Tests**: Unit tests run without filesystem I/O, resulting in 10-100x speedups
2. **Better Caching**: Centralized caching eliminates redundant file reads
3. **Clear Architecture**: Separation of concerns makes code easier to understand and maintain
4. **Future-Proof**: Enables non-filesystem data sources without architectural changes

### Trade-offs

1. **Indirection**: One additional layer between code and filesystem
2. **Learning Curve**: New contributors must understand the abstraction
3. **Upfront Design**: Requires thinking about FileService integration from the start

The benefits significantly outweigh the trade-offs, especially as the codebase grows and test suite expands.

## Enforcement

FileService compliance is enforced through:

1. **Automated Tests**: `TestFileServiceCompliance_*` tests check for violations
2. **Code Review**: Reviewers verify FileService usage in new code
3. **Documentation**: This document provides clear guidance and examples
4. **CI/CD Integration**: Compliance tests run on every commit

Violations are treated as architectural issues and must be resolved before merging.

## Summary

The FileService abstraction is a cornerstone of the LCI architecture. It provides:

- **Consistent** file access patterns across the codebase
- **Testable** components without filesystem dependencies
- **Performant** operations through intelligent caching
- **Maintainable** code with clear separation of concerns

All production code in critical components must use FileService for file operations. Direct filesystem operations are only allowed in explicitly documented exceptions (CLI, configuration, FileService implementation, test infrastructure).

For questions or architectural discussions, refer to this document or consult the FileService implementation in `internal/core/`.
