# Task List: Consolidate ZeroAlloc into FileContentStore

## Task 1: Merge ZeroAllocFileContentStore methods into FileContentStore
**Priority:** High
**Scope:** internal/core/file_content_store.go, internal/core/file_content_store_zero_alloc.go

**Description:** Move all methods from `ZeroAllocFileContentStore` into `FileContentStore` directly. The ZeroAlloc wrapper just embeds `*FileContentStore` and adds methods — those methods should be on the base type. Key methods to move: `GetZeroAllocStringRef`, `GetZeroAllocLine`, `GetZeroAllocLines`, `GetZeroAllocContextLines`, `SearchInLine`, `SearchInLines`, `SearchLinesWithAnyPrefix`, `SearchLinesWithAnySuffix`, `getLineOffsets` (the zero-alloc version that accesses snapshot directly), and all bulk/pattern/analysis/JSON/text-processing helpers.

The private helper `getLineOffsets` in the zero-alloc file accesses `fcs.FileContentStore.snapshot.Load()` — when merged, this becomes simply `fcs.snapshot.Load()`.

Also move all the utility methods: `TrimWhitespaceLines`, `FilterEmptyLines`, `FilterCommentLines`, `ExtractFunctionName`, `ExtractVariableName`, `ExtractImportPath`, `ProcessLinesInBulk`, `FindAllLinesWithPattern`, `FindAllLinesWithPrefix`, `FindAllLinesWithSuffix`, `FindFunctionDefinitions`, `FindVariableDeclarations`, `FindImportStatements`, `FindClassDefinitions`, `LinesToJSON`, `ContextToJSON`, `IsCommentLine`, `IsEmptyLine`, `HasCodeContent`, `ExtractBraceContent`, `ExtractParenContent`, `RemoveCommonPrefix`, `findCommonPrefix`, `CountRunes`, `ExtractRunes`, `FindWithContext`.

**Acceptance Criteria:**
- [ ] All public methods from ZeroAllocFileContentStore exist on FileContentStore
- [ ] Method signatures unchanged (except receiver type changes from ZeroAllocFileContentStore to FileContentStore)
- [ ] All tests in file_content_store_test.go still pass
- [ ] No new imports needed beyond what both files already use
- [ ] The methods compile and are callable on *FileContentStore

**Context:** ZeroAllocFileContentStore is at internal/core/file_content_store_zero_alloc.go. It embeds `*FileContentStore` and adds ~30 methods. The base FileContentStore already has GetContent, GetLine, GetLineCount etc.

---

## Task 2: Update search package consumers to use unified FileContentStore
**Priority:** High
**Scope:** internal/search/semantic_filter_zero_alloc.go, internal/search/context_extractor_zero_alloc.go, internal/search/string_ref_bench_test.go, internal/search/semantic_filter_test_helpers.go

**Description:** Update `ZeroAllocSemanticFilter` and `ZeroAllocContextExtractor` to accept `*core.FileContentStore` directly instead of wrapping it in `*core.ZeroAllocFileContentStore`. Change fields from `zeroAllocStore *core.ZeroAllocFileContentStore` to `store *core.FileContentStore`. Update all method calls from `zasf.zeroAllocStore.GetZeroAllocLine(...)` to `zasf.store.GetZeroAllocLine(...)` etc. Update constructors `NewZeroAllocSemanticFilter` and `NewZeroAllocContextExtractor` to accept `*core.FileContentStore` directly (removing the `NewZeroAllocFileContentStoreFromStore` wrapper call). Update test helpers and benchmarks similarly.

**Acceptance Criteria:**
- [ ] `ZeroAllocSemanticFilter.zeroAllocStore` field changed to `store *core.FileContentStore`
- [ ] `ZeroAllocContextExtractor.zeroAllocStore` field changed to `store *core.FileContentStore`
- [ ] All constructors accept `*core.FileContentStore` directly
- [ ] All method calls updated (no more intermediate wrapper)
- [ ] `go test ./internal/search/...` passes

**Context:** These are the only direct consumers of ZeroAllocFileContentStore. After Task 1 merges the methods, these just need their field types and constructor calls updated.

---

## Task 3: Remove ZeroAllocFileContentStore wrapper class and update tests
**Priority:** High
**Scope:** internal/core/file_content_store_zero_alloc.go, internal/core/file_content_store_zero_alloc_test.go

**Description:** Delete the `ZeroAllocFileContentStore` struct, all its constructor functions (`NewZeroAllocFileContentStore`, `NewZeroAllocFileContentStoreFromStore`, `NewZeroAllocFileContentStoreWithLimit`), and the file `file_content_store_zero_alloc.go` itself since all methods were moved to `file_content_store.go` in Task 1. Update tests from `file_content_store_zero_alloc_test.go` — change them to test methods on `*FileContentStore` directly. Either merge into `file_content_store_test.go` or keep as a separate test file but calling base type methods.

**Acceptance Criteria:**
- [ ] `ZeroAllocFileContentStore` struct no longer exists
- [ ] `file_content_store_zero_alloc.go` deleted
- [ ] No compilation errors across the project
- [ ] All zero-alloc tests preserved but test the base FileContentStore
- [ ] `go test ./internal/core/...` passes
- [ ] `go test ./internal/search/...` passes

**Context:** After Tasks 1 and 2, this struct has no consumers. Its methods live on FileContentStore. Clean deletion.

---

## Task 4: Run full test suite and verify no regressions
**Priority:** High
**Scope:** all packages

**Description:** Run the complete test suite to verify no regressions from the consolidation. Run: `go test ./... -count=1 -timeout 300s`. Fix any compilation or test failures. Verify the workflow scenario tests still pass. Grep the entire codebase to confirm no references to ZeroAllocFileContentStore remain (except possibly in git history comments).

**Acceptance Criteria:**
- [ ] `go test ./...` passes with zero failures
- [ ] No new compilation warnings
- [ ] Workflow scenario tests pass (`go test ./internal/mcp/workflow_scenarios/... -count=1 -timeout 300s`)
- [ ] `grep -r ZeroAllocFileContentStore internal/` returns zero results
- [ ] No dead imports referencing removed types

**Context:** This is the verification task. The baseline workflow test suite took ~70s total with all scenarios passing.
