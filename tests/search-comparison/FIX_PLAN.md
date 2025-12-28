# Search Comparison Test Suite - Fix Plan

## Issues Discovered

### 1. **LCI Output Format Mismatch**
**Problem**: The test parsing expects `filepath:line: content` but LCI outputs a different format:
```
=== Direct Matches ===
auth.go:20
    20 | func NewAuthService(userService *UserService) *AuthService {
    21 | 	return &AuthService{userService: userService}
    22 | }
```

**Impact**: Parser creates invalid SearchResult entries with garbage file paths like `"10 |     std"`, `"2025/10/22 11"`, etc.

### 2. **Debug/Logging Output Not Filtered**
**Problem**: LCI outputs extensive debug information:
- `[INDEX-INFO]` lines with lock operations
- `2025/10/22 HH:MM:SS` timestamp lines
- Performance metrics
- Index statistics

**Impact**: These lines are being parsed as search results, creating noise.

### 3. **Working Directory Issue**
**Problem**: When running `lci` with `cmd.Dir = fixtureDir`, the tool needs to execute from within the target directory.

**Impact**: Initial test showed 0 files indexed when using `--root` flag from project root.

### 4. **Output Format Sections**
**Problem**: LCI output has multiple sections:
- Index info (should skip)
- "Found X results" summary line
- "Total matches" line
- "=== Direct Matches ===" header
- Actual results with context

**Impact**: Need to parse only the actual results section.

## Fix Strategy

### Phase 1: Update Output Parsing ✅

1. **Filter stderr vs stdout**
   - Run with separate stderr capture
   - Filter `[INDEX-INFO]` and timestamp lines from stderr

2. **Parse LCI-specific format**
   ```go
   // Expected format:
   // filename:linenum
   //     linenum | content
   //     linenum | content
   ```

3. **Extract file:line pairs correctly**
   - First line of each result block: `filename:linenum`
   - Context lines: `  linenum | content`
   - Need to track current file across context lines

### Phase 2: Fix Test Execution ✅

1. **Change working directory approach**
   ```go
   cmd := exec.Command(lciBinary, "search", pattern)
   cmd.Dir = fixtureDir // Already correct
   ```

2. **Add stderr filtering**
   ```go
   var stdout, stderr bytes.Buffer
   cmd.Stdout = &stdout
   cmd.Stderr = &stderr
   // Parse only stdout
   ```

3. **Handle exit codes properly**
   - Exit 0: Success with results
   - Exit 1: Success with no results (not an error)
   - Other: Actual errors

### Phase 3: Test Case Refinement ✅

1. **Case-sensitivity fixes**
   - Change `"getUser"` → `"GetUser"` (Go convention)
   - Change `"authenticate"` → case-insensitive or language-specific

2. **Pattern adjustments**
   - Ensure patterns match across all language samples
   - Document language-specific naming conventions

3. **Expected results documentation**
   - Document what each pattern should find
   - Note differences between text search and semantic search

### Phase 4: Assertion Strategy ✅

1. **File-level coverage focus**
   - Primary assertion: MCP finds all files that grep/rg find
   - Secondary: MCP may find additional files (semantic understanding)

2. **Line-level comparison** (optional)
   - For key test cases, verify line numbers match
   - Allow for context lines in MCP results

3. **Result normalization**
   - Strip whitespace
   - Normalize file paths (handle `./` prefix)
   - Sort for deterministic comparison

## Implementation Plan

### Step 1: Fix Parser Function
```go
func runMCPSearch(t *testing.T, fixtureDir, pattern string) SearchResults {
    lciBinary, err := findLciBinary()
    require.NoError(t, err)

    cmd := exec.Command(lciBinary, "search", pattern)
    cmd.Dir = fixtureDir

    // Separate stdout/stderr
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err = cmd.Run()
    // Allow exit code 1 (no results)
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
            t.Fatalf("lci failed: %v\nstderr: %s", err, stderr.String())
        }
    }

    return parseLCIOutput(stdout.String())
}

func parseLCIOutput(output string) SearchResults {
    results := make(SearchResults, 0)
    lines := strings.Split(output, "\n")

    var currentFile string
    var currentLine int
    inResults := false

    for _, line := range lines {
        // Skip until we hit results section
        if strings.HasPrefix(line, "=== Direct Matches ===") {
            inResults = true
            continue
        }
        if !inResults {
            continue
        }

        // Skip empty lines
        if strings.TrimSpace(line) == "" {
            continue
        }

        // Check for new result: "filename:linenum"
        if !strings.HasPrefix(line, "    ") {
            parts := strings.SplitN(line, ":", 2)
            if len(parts) == 2 {
                currentFile = parts[0]
                fmt.Sscanf(parts[1], "%d", &currentLine)

                // Add the primary result
                results = append(results, SearchResult{
                    FilePath: currentFile,
                    Line:     currentLine,
                })
            }
        }
        // Context lines: "    linenum | content"
        // We capture these for completeness but focus on file:line
    }

    return results
}
```

### Step 2: Update Test Patterns
```go
testCases := []TestCase{
    {
        Name:        "Type definition",
        Pattern:     "UserService",
        Description: "Search for UserService type/class",
        Language:    "all",
    },
    {
        Name:        "Function name - Go",
        Pattern:     "GetUser",
        Description: "Search for GetUser function (capitalized)",
        Language:    "go",
    },
    {
        Name:        "Function name - JS",
        Pattern:     "getUser",
        Description: "Search for getUser function (camelCase)",
        Language:    "js",
    },
    // etc.
}
```

### Step 3: Improve Assertions
```go
// Focus on file-level coverage
mcpFiles := mcpResults.FilePaths()
grepFiles := grepResults.FilePaths()

// MCP should find at least the files grep found
for _, grepFile := range grepFiles {
    assert.Contains(t, mcpFiles, grepFile,
        "MCP should find file %s that grep found", grepFile)
}

// Log if MCP found additional files (not an error)
extraFiles := difference(mcpFiles, grepFiles)
if len(extraFiles) > 0 {
    t.Logf("MCP found additional files (semantic): %v", extraFiles)
}
```

### Step 4: Add Helper Functions
```go
// difference returns elements in a that are not in b
func difference(a, b []string) []string {
    mb := make(map[string]bool, len(b))
    for _, x := range b {
        mb[x] = true
    }
    var diff []string
    for _, x := range a {
        if !mb[x] {
            diff = append(diff, x)
        }
    }
    return diff
}
```

## Expected Outcomes

### Success Criteria
1. ✅ All tests parse LCI output correctly
2. ✅ No debug/logging lines in results
3. ✅ File paths are clean and valid
4. ✅ MCP finds at least what grep/rg find
5. ✅ Test reports show meaningful comparisons

### Performance Expectations
- Test suite should complete in < 10 seconds
- Each fixture indexed in < 100ms
- Search operations in < 10ms

### Known Differences (Not Errors)
1. **MCP may find more**: Semantic understanding can find additional references
2. **Context lines differ**: MCP provides more context by default
3. **Ordering may differ**: Different tools have different sort orders

## Testing the Fixes

```bash
# 1. Build binary
make build

# 2. Test single fixture manually
cd tests/search-comparison/fixtures/go-sample
../../../../lci search "UserService"

# 3. Run one test case
cd ../..
go test -v -run "TestSearchComparison/Type_definition"

# 4. Run full suite
make test-search-comparison

# 5. Check generated reports
ls test-reports/
cat test-reports/Type_definition.json
```

## Timeline

- **Step 1 (Parser Fix)**: 30 minutes
- **Step 2 (Test Patterns)**: 15 minutes
- **Step 3 (Assertions)**: 15 minutes
- **Step 4 (Helpers)**: 10 minutes
- **Testing & Validation**: 20 minutes

**Total**: ~90 minutes

## Risks & Mitigations

### Risk: LCI output format changes
**Mitigation**: Add output format tests as guard rails

### Risk: Language-specific patterns don't match
**Mitigation**: Create language-specific test cases with appropriate patterns

### Risk: Flaky tests due to ordering
**Mitigation**: Sort and normalize all results before comparison

## Future Enhancements

1. **Regex pattern support**: Test regex patterns in addition to plain text
2. **Case-insensitive flag**: Add test cases with case-insensitive searches
3. **Performance regression**: Track search times across test runs
4. **Coverage metrics**: Measure what % of codebase features are tested
