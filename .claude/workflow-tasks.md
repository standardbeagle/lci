# Minimal CLI/MCP Architecture - Server-Only Indexing

## Overview
Refactor LCI so all indexing happens on the server. CLI commands should only use server APIs.
The directive: "only server indexes are allowed, the only local functions should be index management functions"

## Current State Analysis
- Server exists with endpoints: `/status`, `/search`, `/symbol`, `/fileinfo`, `/shutdown`, `/ping`, `/reindex`
- Only `grepCommand` uses `ensureServerRunning()`
- `Before` hook still does local indexing for: search, grep, def, refs, stats, tree, unroll, git-analyze
- `/symbol` and `/fileinfo` return "not implemented"

---

## Task 1: Remove Local Indexing from Before Hook
**Priority:** High
**Scope:** cmd/lci/main.go (1 file)
**Description:** Remove the local indexing logic from the `Before` hook for commands that should use the server. Keep only index management commands (like test-run, daemon mode) with local indexing.

**Acceptance Criteria:**
- [ ] Remove `indexCommands` map entries for: search, grep, def, refs, tree, unroll, git-analyze
- [ ] Keep only index management logic (daemon, test-run, mcp)
- [ ] Remove the "Indexed X files (Y symbols)" message for server-based commands
- [ ] Verify Before hook no longer creates local indexer for server commands

**Context:** Lines 776-812 contain the problematic local indexing in Before hook.

---

## Task 2: Add Missing Server Endpoints - Stats
**Priority:** High
**Scope:** internal/server/server.go, internal/server/types.go (2 files)
**Description:** Add `/stats` endpoint to server to return index statistics (file count, symbol count, memory usage, etc.)

**Acceptance Criteria:**
- [ ] Add `StatsRequest` and `StatsResponse` types
- [ ] Add `handleStats` handler in server.go
- [ ] Register `/stats` endpoint in `registerHandlers`
- [ ] Return meaningful statistics from the indexer

**Context:** The stats command needs this to show index information without local indexer.

---

## Task 3: Add Missing Server Endpoints - Definition
**Priority:** High
**Scope:** internal/server/server.go, internal/server/client.go, internal/server/types.go (3 files)
**Description:** Add `/definition` endpoint to find symbol definitions by name.

**Acceptance Criteria:**
- [ ] Add `DefinitionRequest` and `DefinitionResponse` types
- [ ] Add `handleDefinition` handler that searches for definitions
- [ ] Add `GetDefinition` method to client.go
- [ ] Register `/definition` endpoint

**Context:** The `def` command needs this to find where symbols are defined.

---

## Task 4: Add Missing Server Endpoints - References
**Priority:** High
**Scope:** internal/server/server.go, internal/server/client.go, internal/server/types.go (3 files)
**Description:** Add `/references` endpoint to find symbol references.

**Acceptance Criteria:**
- [ ] Add `ReferencesRequest` and `ReferencesResponse` types
- [ ] Add `handleReferences` handler that finds all references
- [ ] Add `GetReferences` method to client.go
- [ ] Register `/references` endpoint

**Context:** The `refs` command needs this to find where symbols are used.

---

## Task 5: Add Missing Server Endpoints - Tree
**Priority:** Medium
**Scope:** internal/server/server.go, internal/server/client.go, internal/server/types.go (3 files)
**Description:** Add `/tree` endpoint to generate file/symbol tree view.

**Acceptance Criteria:**
- [ ] Add `TreeRequest` and `TreeResponse` types
- [ ] Add `handleTree` handler that generates tree output
- [ ] Add `GetTree` method to client.go
- [ ] Register `/tree` endpoint

**Context:** The `tree` command needs this to show project structure.

---

## Task 6: Refactor searchCommand to Use Server
**Priority:** High
**Scope:** cmd/lci/search.go (1 file)
**Description:** Refactor `searchCommand` to use server client instead of local indexer.

**Acceptance Criteria:**
- [ ] Call `ensureServerRunning(cfg)` at start
- [ ] Use `client.Search()` instead of local search
- [ ] Remove references to local indexer
- [ ] Maintain same output format

**Context:** Currently uses app.Metadata["indexer"] which is the local indexer.

---

## Task 7: Refactor definitionCommand to Use Server
**Priority:** High
**Scope:** cmd/lci/main.go (definitionCommand function)
**Description:** Refactor `definitionCommand` to use the new server `/definition` endpoint.

**Acceptance Criteria:**
- [ ] Call `ensureServerRunning(cfg)` at start
- [ ] Use `client.GetDefinition()` instead of local search
- [ ] Remove references to local indexer
- [ ] Maintain same output format

**Context:** Lines ~1114-1135 in main.go

---

## Task 8: Refactor referencesCommand to Use Server
**Priority:** High
**Scope:** cmd/lci/main.go (referencesCommand function)
**Description:** Refactor `referencesCommand` to use the new server `/references` endpoint.

**Acceptance Criteria:**
- [ ] Call `ensureServerRunning(cfg)` at start
- [ ] Use `client.GetReferences()` instead of local search
- [ ] Remove references to local indexer
- [ ] Maintain same output format

**Context:** Lines ~1136-1167 in main.go

---

## Task 9: Refactor treeCommand to Use Server
**Priority:** Medium
**Scope:** cmd/lci/main.go (treeCommand function)
**Description:** Refactor `treeCommand` to use the new server `/tree` endpoint.

**Acceptance Criteria:**
- [ ] Call `ensureServerRunning(cfg)` at start
- [ ] Use `client.GetTree()` instead of local indexer
- [ ] Remove references to local indexer
- [ ] Maintain same output format

**Context:** Lines ~1183-1233 in main.go

---

## Task 10: Add Client Stats Method and Refactor statsCommand
**Priority:** High
**Scope:** internal/server/client.go, cmd/lci/status.go (2 files)
**Description:** Add `GetStats` method to client and refactor status command.

**Acceptance Criteria:**
- [ ] Add `GetStats()` method to client.go
- [ ] Refactor statusCommand to use server stats
- [ ] Remove local indexer stats access
- [ ] Ensure server shows accurate file/symbol counts

**Context:** The status command currently uses local indexer which shows incorrect counts.

---

## Task 11: Update Integration Tests
**Priority:** High
**Scope:** internal/server/server_integration_test.go, tests/search-comparison/*.go (3-4 files)
**Description:** Update tests to verify server-only architecture works correctly.

**Acceptance Criteria:**
- [ ] Add tests for new endpoints (stats, definition, references, tree)
- [ ] Verify CLI commands work without local indexing
- [ ] Ensure search-comparison tests pass
- [ ] Test that "Indexed 0 files" message no longer appears for server commands

**Context:** Tests should verify the architectural change is complete.

---

## Task 12: Clean Up Unused Code
**Priority:** Low
**Scope:** cmd/lci/main.go, internal/indexing/*.go (2-3 files)
**Description:** Remove any orphaned local indexing code that's no longer needed.

**Acceptance Criteria:**
- [ ] Remove unused indexer creation in Before hook
- [ ] Clean up app.Metadata["indexer"] if no longer used
- [ ] Remove any dead code paths
- [ ] Ensure no duplicate indexing code remains

**Context:** Final cleanup after all commands are refactored.

---

## Summary
- **Total Tasks:** 12
- **High Priority:** 9
- **Medium Priority:** 2
- **Low Priority:** 1
- **Estimated Files Changed:** ~10-15 files
