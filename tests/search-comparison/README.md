# Search Comparison Test Suite

This test suite compares Lightning Code Index (LCI) MCP search results with traditional search tools (grep and ripgrep) across multiple programming languages.

## Overview

The search comparison suite validates that LCI's semantic search capabilities provide at least the same file coverage as traditional text-based search tools, while potentially offering additional context and semantic understanding.

## Test Structure

```
tests/search-comparison/
├── README.md                    # This file
├── comparison_test.go           # Original comparison tests (basic patterns)
├── enhanced_comparison_test.go  # NEW: Comprehensive 80+ test cases
├── stress_test.go              # NEW: Performance and stress tests
├── COMPARISON_RESULTS.md       # NEW: Detailed test results and findings
├── fixtures/                   # Sample codebases for testing
│   ├── go-sample/              # Go sample project
│   ├── js-sample/              # JavaScript sample project
│   ├── python-sample/          # Python sample project
│   ├── rust-sample/            # Rust sample project
│   ├── cpp-sample/             # C++ sample project
│   └── java-sample/            # Java sample project
└── test-reports/               # Generated comparison reports (gitignored)
```

## Supported Languages

Each language fixture includes common patterns for testing:

- **Go**: Interfaces, error handling, structs
- **JavaScript**: Classes, async functions, modules
- **Python**: Type hints, classes, error handling
- **Rust**: Traits, error handling, structs
- **C++**: Classes, templates, exceptions
- **Java**: Interfaces, classes, exceptions

## Test Cases

The suite includes tests for:

1. **Function Name Search**: Finding common function names (e.g., `getUser`)
2. **Class/Struct Definitions**: Finding type definitions (e.g., `UserService`)
3. **Keyword Search**: Case-insensitive searches (e.g., `authenticate`)
4. **Error Messages**: Finding error strings (e.g., `invalid credentials`)
5. **Interface/Trait Definitions**: Finding abstract types (e.g., `Database`)
6. **Language-Specific Patterns**:
   - Go: Error return patterns
   - JavaScript: Async functions
   - Python: Type hints with `Optional`

## Running the Tests

### Run all comparison tests
```bash
cd tests/search-comparison
go test -v
```

### Run specific test suites
```bash
# Original comparison tests
go test -v -run "TestSearchComparison"

# NEW: Enhanced comparison tests (80+ test cases)
go test -v -run "TestEnhancedGrepComparison"

# NEW: Run specific test categories
go test -v -run "TestEnhancedGrepComparison/Literal"
go test -v -run "TestEnhancedGrepComparison/Special"
go test -v -run "TestEnhancedGrepComparison/Multi-word"

# NEW: Performance tests
go test -v -run "TestGrepPerformanceComparison"

# NEW: Edge case tests
go test -v -run "TestEdgeCasePatterns"
```

### Run with coverage
```bash
go test -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### From project root with Makefile
```bash
# Add to Makefile
make test-search-comparison
```

### Quick reference for new tests

**Enhanced Comparison Tests** (`enhanced_comparison_test.go`):
- 80+ comprehensive test cases
- Special character handling
- Edge cases and Unicode
- Language-specific patterns

**Stress Tests** (`stress_test.go`):
- Performance benchmarking
- Large pattern sets
- Memory and throughput testing

See **COMPARISON_RESULTS.md** for detailed findings and known issues.

## Requirements

### Required Tools
- **Go 1.21+**: For running the tests
- **grep**: Standard Unix text search (usually pre-installed)
- **lci binary**: Built from project (`make build`)

### Optional Tools
- **ripgrep (rg)**: For additional comparison (tests will skip rg comparisons if not installed)
  - Install: `cargo install ripgrep` or via package manager

## Test Output

### Console Output
Each test logs:
- Pattern being searched
- Number of matches found by each tool
- File paths matched by each tool
- Comparison results

Example:
```
=== RUN   TestSearchComparison/Simple_function_name
    comparison_test.go:XXX: Pattern: getUser
    comparison_test.go:XXX: Description: Search for a common function name across languages
    comparison_test.go:XXX: MCP results: 6 matches
    comparison_test.go:XXX: grep results: 6 matches
    comparison_test.go:XXX: ripgrep results: 6 matches
    comparison_test.go:XXX: MCP files: [auth.go main.go auth.js index.js auth.py main.py]
    comparison_test.go:XXX: grep files: [auth.go main.go auth.js index.js auth.py main.py]
    comparison_test.go:XXX: ripgrep files: [auth.go main.go auth.js index.js auth.py main.py]
--- PASS: TestSearchComparison/Simple_function_name (0.15s)
```

### JSON Reports
Detailed comparison reports are saved to `test-reports/` directory:

```json
{
  "test_case": "Simple function name",
  "pattern": "getUser",
  "language": "all",
  "mcp_count": 6,
  "grep_count": 6,
  "ripgrep_count": 6,
  "mcp_files": ["auth.go", "main.go", "..."],
  "grep_files": ["auth.go", "main.go", "..."],
  "ripgrep_files": ["auth.go", "main.go", "..."]
}
```

## Assertions

The test suite validates:

1. **File Coverage**: MCP finds at least all files that grep finds
2. **File Coverage vs Ripgrep**: MCP finds at least all files that ripgrep finds
3. **No False Negatives**: MCP doesn't miss files that traditional tools find

**Note**: The test suite focuses on file-level coverage rather than exact match counts, as LCI may provide additional semantic matches that text-based tools miss.

## Extending the Test Suite

### Adding New Test Cases

Edit `comparison_test.go` and add to the `testCases` slice:

```go
{
    Name:        "Your test name",
    Pattern:     "search_pattern",
    Description: "What this test validates",
    Language:    "all", // or specific: "go", "js", "python", etc.
}
```

### Adding New Language Fixtures

1. Create a new directory: `fixtures/newlang-sample/`
2. Add sample code files with common patterns
3. Update `getFixturePath()` function in `comparison_test.go`
4. Add language-specific test cases

### Sample Code Guidelines

Each language fixture should include:
- User/entity management code
- Authentication/authorization code
- Database/storage abstractions
- Error handling patterns
- Common language idioms

## Integration with CI/CD

Add to your CI pipeline:

```yaml
- name: Run Search Comparison Tests
  run: |
    make build
    cd tests/search-comparison
    go test -v
```

## Troubleshooting

### "MCP search failed" errors
- Ensure `lci` binary is built: `make build`
- Check that the binary is in the expected location relative to test files

### Ripgrep not found
- Tests will skip ripgrep comparisons if `rg` is not in PATH
- This is not an error; tests will still compare MCP vs grep

### File path mismatches
- Ensure fixture directories use relative paths consistently
- Check that `.git` directories are excluded from searches

## Performance Considerations

- **Index Build Time**: Each test case rebuilds the index (CLI pattern)
- **Small Fixtures**: Test fixtures are intentionally small for fast execution
- **Parallel Execution**: Tests can run in parallel with `-parallel` flag

## Key Findings from Enhanced Tests

### ❌ Critical Issues Discovered

1. **Configuration Files Not Indexed**
   - `go.mod`, `package.json`, `Cargo.toml` not indexed by lci grep
   - Impact: Missing matches that grep/rg find
   - See COMPARISON_RESULTS.md Issue #1

2. **Dot Character Treatment**
   - lci grep: treats `.` as literal (56 matches)
   - grep/rg: treats `.` as regex wildcard (540 matches)
   - Major behavioral incompatibility
   - See COMPARISON_RESULTS.md Issue #2

3. **Special Character Handling**
   - Patterns like `[]`, `{}` cause failures in grep/rg
   - Need explicit literal/regex mode distinction
   - See COMPARISON_RESULTS.md Issues #3-4

### ✅ What Works Well

- Basic literal patterns
- Multi-word exact matches
- Case-sensitive searches
- Language-specific keywords
- Numeric and operator patterns

### 📊 Test Coverage

- **80+ test cases** across multiple categories
- **6 languages** (Go, JS, Python, Rust, C++, Java)
- **Performance benchmarks** included
- **Edge cases** (Unicode, long patterns, etc.)

For complete analysis, see **COMPARISON_RESULTS.md**

## Future Enhancements

Potential improvements:
- [ ] Add regex pattern tests
- [ ] Test case-sensitive vs case-insensitive searches
- [ ] Compare result ranking/relevance
- [ ] Test multi-pattern searches
- [ ] Add performance regression detection
- [ ] Test incremental index updates
- [ ] Compare semantic vs text-only results
- [ ] Fix configuration file indexing
- [ ] Add `-F` literal mode flag
- [ ] Add `-E` regex mode flag
- [ ] Add context line support (`-A`, `-B`, `-C`)

## Related Documentation

- [LCI Testing Strategy](/docs/testing-strategy.md)
- [MCP Server Documentation](/internal/mcp/README.md)
- [Search Implementation](/internal/search/README.md)
- **[COMPARISON_RESULTS.md](./COMPARISON_RESULTS.md)** - Detailed test results and findings
