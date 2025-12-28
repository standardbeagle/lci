# Bugs Found During Search Comparison Testing

## Summary

The search comparison test suite has uncovered **critical line number discrepancies** in `lci grep` output. These tests validate that `lci grep` (trigram-based text search) matches standard `grep` exactly, which is a fundamental requirement.

## Bug #1: Off-by-One Error in Line Reporting

**Status**: 🔴 **CRITICAL BUG**
**Component**: `lci grep` command
**Severity**: High - affects correctness of search results

### Description

`lci grep` consistently reports line numbers that are off by 1 (reports line N-1 when the match is actually on line N) for certain patterns.

### Reproduction

```bash
cd tests/search-comparison/fixtures/rust-sample
grep -rn "invalid credentials" .
# Output: ./src/auth.rs:24:            return Err("invalid credentials".into());

../../../../lci grep "invalid credentials"
# Output: src/auth.rs:23:24:            return Err("invalid credentials".into());
#                     ^^ should be 24, not 23
```

### Expected Behavior

`lci grep` line numbers should match `grep` line numbers exactly:
- grep: `auth.rs:24`
- lci grep: should output `auth.rs:24` (currently outputs `auth.rs:23`)

### Actual File Content

```rust
Line 22:     pub fn authenticate(&self, username: &str, password: &str) -> Result<Token, Box<dyn Error>> {
Line 23:         if username.is_empty() || password.is_empty() {
Line 24:             return Err("invalid credentials".into());  // ← MATCH IS HERE
Line 25:         }
```

The match is definitively on **line 24**.

### Impact

- ❌ **Test Failures**: 5 out of 8 test cases failing due to line number mismatches
- ❌ **User Confusion**: IDE integration and jump-to-line will go to wrong location
- ❌ **Correctness**: Violates the core promise that `lci grep` matches `grep` exactly

### Affected Test Cases

1. ✅ **PASS**: Go - GetUser function (2/2 matches)
2. ✅ **PASS**: JavaScript - async keyword (3/3 matches)
3. ✅ **PASS**: Python - type hints (5/5 matches)
4. ❌ **FAIL**: Type definition - UserService (file count matches, line numbers off)
5. ❌ **FAIL**: Type definition - AuthService (file count matches, line numbers off)
6. ❌ **FAIL**: Interface/trait - Database (2 line mismatches in Rust file)
7. ❌ **FAIL**: Error string - "invalid credentials" (1 line mismatch in Rust file)
8. ❌ **FAIL**: Error string - "invalid token" (1 line mismatch in Rust file)

### Pattern

- ✅ Works correctly for: Simple single-word patterns in some languages
- ❌ Fails for: Multi-word patterns, Rust files, certain code structures

### Test Output Examples

```
# Error string - invalid credentials
lci grep results: 6 matches
grep results: 6 matches
✓ File coverage matches (all 6 files found)
✗ Line number mismatch at rust-sample/src/auth.rs:24 (lci reports 23)
```

```
# Interface/trait - Database
lci grep results: 9 matches
grep results: 20 matches
✓ File coverage matches (5 files)
✗ Line mismatches at:
  - rust-sample/src/main.rs:17 (lci reports different line)
  - rust-sample/src/main.rs:21 (lci reports different line)
```

## Required Fix

### Location
Likely in: `internal/search/grep.go` or related line counting logic

### Fix Requirements
1. Ensure line counting uses 1-based indexing (like grep)
2. Verify line number calculation for multi-line matches
3. Test with various file encodings and line endings
4. Validate against the authoritative grep implementation

### Verification
After fix, all 8 test cases in `tests/search-comparison/` should pass with:
```bash
make test-search-comparison
# Expected: PASS for all tests with exact line number matches
```

## Additional Notes

### Test Suite Value

This test suite has proven its value by:
1. ✅ **Detecting Real Bugs**: Found critical line numbering issues
2. ✅ **Preventing Regressions**: Will catch future breakage
3. ✅ **Validating Correctness**: Ensures grep-compatibility promise is kept
4. ✅ **Multi-Language Coverage**: Tests across 6 programming languages

### Why This Matters

The `lci grep` command is explicitly marketed as:
> "Ultra-fast text search (40% faster, 75% less memory)"

This implies **drop-in replacement for grep**, which requires:
- ✅ Same files found (currently working)
- ✅ Same match counts (currently working for most cases)
- ❌ **Same line numbers** (FAILING - this bug)
- ✅ Same content (currently working)

**Without exact line number parity, `lci grep` cannot be considered a grep replacement.**

## Recommendation

**Priority**: 🔴 **P0 - Must fix before any release**

This bug breaks a fundamental promise of the tool and will cause user frustration when integrated with editors/IDEs.

---

**Discovered**: 2025-10-22
**Test Suite**: `tests/search-comparison/`
**Reported By**: Automated test comparison framework
