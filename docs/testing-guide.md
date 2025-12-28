# Lightning Code Index - Testing Suite

This directory contains all testing, benchmarking, and evaluation scripts for the Lightning Code Index.

## Directory Structure

### `/integration/`
Integration test scripts and end-to-end validation:
- `test_semantic_complete.go` - Semantic annotation system integration tests
- Production test suites and validation scripts

### `/mcp/` 
MCP (Model Context Protocol) specific tests:
- MCP server integration tests
- Tool validation and protocol compliance tests

### `/performance/`
Performance evaluation and profiling:
- Memory usage analysis
- Search performance benchmarks
- Scalability tests

### `/benchmarks/`
Benchmark executables and performance testing:
- Binary test executables (`.test` files)
- Performance comparison scripts
- Load testing utilities

### `/reports/`
Test results, reports, and analysis artifacts:
- JSON test results
- Performance reports
- Evaluation summaries
- Test execution logs

## Running Tests

### Integration Tests
```bash
# Run Go integration tests
go test ./...

# Run Python MCP integration tests  
cd testing/integration
python test_*.py
```

### Performance Benchmarks
```bash
# Run performance benchmarks
cd testing/benchmarks
./lightning-code-index.test -bench=.

# Run realistic workload benchmark
cd testing/performance
python realistic_benchmark.py
```

### MCP Tests
```bash
# Test MCP server functionality
cd testing/mcp
python mcp_test_client.py
```

## Test Data

Test fixtures and sample codebases are located in:
- `internal/*/testdata/` - Language-specific test data
- `testdata/` - General test fixtures

## Contributing

When adding new tests:
1. Place integration tests in `/integration/`
2. Add performance tests in `/performance/`
3. Put language-specific tests near the related code in `internal/*/`
4. Document test scenarios and expected outcomes
5. Follow the existing naming conventions
 6. Run `make test-catalog` to regenerate the centralized test catalog

## Test Catalog & Naming Conventions

An automatically generated catalog of all Go test and benchmark functions (grouped by category and feature tags) is maintained at `testing/TEST_CATALOG.md`.

Update it after modifying tests:

```bash
make test-catalog
```

See `docs/TEST_NAMING_CONVENTIONS.md` for required naming/documentation patterns.

## Cleanup

This directory structure was created to organize previously scattered test files throughout the project root. All test artifacts should now be contained within this `testing/` directory.