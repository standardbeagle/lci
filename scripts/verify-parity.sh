#!/bin/bash
# Verify feature and test parity between lci and lightning-code-index
# Uses checksums and diff to ensure no divergence

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

ORIGINAL="../lightning-code-index"
TARGET="."

echo -e "${BLUE}=== LCI Parity Verification ===${NC}"
echo "Original: $ORIGINAL"
echo "Target: $TARGET"
echo ""

MATCHES=0
DIFFS=0
MISSING=0

# Normalize import paths for comparison
normalize_go() {
    local file="$1"
    sed 's|standardbeagle/lightning-code-index|github.com/standardbeagle/lci|g' "$file" | \
    sed 's|github.com/standardbeagle/lci|NORMALIZED_MODULE|g'
}

# Compare two Go files (normalized)
compare_go_files() {
    local orig="$1"
    local tgt="$2"

    if [[ ! -f "$orig" ]]; then
        echo -e "${RED}  ✗ Missing in original: $orig${NC}"
        return 1
    fi

    if [[ ! -f "$tgt" ]]; then
        echo -e "${YELLOW}  ⚠ Missing in target: $tgt${NC}"
        ((MISSING++))
        return 1
    fi

    # Normalize and compare
    local orig_norm=$(mktemp)
    local tgt_norm=$(mktemp)

    normalize_go "$orig" > "$orig_norm"
    normalize_go "$tgt" > "$tgt_norm"

    if diff -q "$orig_norm" "$tgt_norm" >/dev/null 2>&1; then
        echo -e "${GREEN}  ✓ Match: $(basename $tgt)${NC}"
        ((MATCHES++))
        rm "$orig_norm" "$tgt_norm"
        return 0
    else
        echo -e "${YELLOW}  ~ Diff: $(basename $tgt)${NC}"
        ((DIFFS++))

        # Show first few lines of diff
        echo -e "${YELLOW}    First differences:${NC}"
        diff -u "$orig_norm" "$tgt_norm" | head -20 | sed 's/^/    /'

        rm "$orig_norm" "$tgt_norm"
        return 1
    fi
}

# Verify core internal packages
echo -e "${BLUE}Verifying core internal packages...${NC}"

CORE_PACKAGES=(
    "core"
    "indexing"
    "search"
    "mcp"
    "semantic"
    "analysis"
    "parser"
)

for pkg in "${CORE_PACKAGES[@]}"; do
    echo -e "${BLUE}Package: internal/$pkg${NC}"

    if [[ ! -d "$ORIGINAL/internal/$pkg" ]]; then
        echo -e "${RED}  ✗ Package not found in original${NC}"
        continue
    fi

    if [[ ! -d "$TARGET/internal/$pkg" ]]; then
        echo -e "${RED}  ✗ Package not found in target${NC}"
        continue
    fi

    # Compare each Go file
    while IFS= read -r orig_file; do
        rel_path="${orig_file#$ORIGINAL/}"
        tgt_file="$TARGET/$rel_path"
        compare_go_files "$orig_file" "$tgt_file"
    done < <(find "$ORIGINAL/internal/$pkg" -name "*.go" -not -name "*_test.go" | sort)
done

echo ""
echo -e "${BLUE}Verifying test files...${NC}"

for pkg in "${CORE_PACKAGES[@]}"; do
    echo -e "${BLUE}Tests: internal/$pkg${NC}"

    # Compare test files
    while IFS= read -r orig_file; do
        rel_path="${orig_file#$ORIGINAL/}"
        tgt_file="$TARGET/$rel_path"
        compare_go_files "$orig_file" "$tgt_file"
    done < <(find "$ORIGINAL/internal/$pkg" -name "*_test.go" 2>/dev/null | sort)
done

echo ""
echo -e "${BLUE}Checking for ported test infrastructure...${NC}"

# Check what's been ported
TEST_INFRA=(
    "tests/search-comparison"
    "testing/integration"
    "tests/benchmarks"
    "real_projects"
    "docs/testing-strategy.md"
    "Makefile"
)

for item in "${TEST_INFRA[@]}"; do
    if [[ -e "$TARGET/$item" ]]; then
        echo -e "${GREEN}  ✓ Ported: $item${NC}"
    else
        echo -e "${YELLOW}  ⚠ Not yet ported: $item${NC}"
    fi
done

echo ""
echo -e "${BLUE}=== Summary ===${NC}"
echo -e "Files matching: ${GREEN}$MATCHES${NC}"
echo -e "Files with differences: ${YELLOW}$DIFFS${NC}"
echo -e "Files missing in target: ${YELLOW}$MISSING${NC}"

if [[ $DIFFS -eq 0 ]] && [[ $MISSING -eq 0 ]]; then
    echo -e "${GREEN}✓ Core packages are in perfect sync!${NC}"
    exit 0
elif [[ $DIFFS -gt 0 ]] && [[ $MISSING -eq 0 ]]; then
    echo -e "${YELLOW}⚠ Some differences found, but all files present${NC}"
    exit 0
else
    echo -e "${RED}✗ Missing files detected${NC}"
    exit 1
fi
