#!/bin/bash
# Safe porting script from lightning-code-index to lci
# Uses verification techniques to prevent copy errors

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Paths
ORIGINAL="../lightning-code-index"
TARGET="."
LOG_FILE="port-manifest-$(date +%Y%m%d-%H%M%S).log"

# Counters
PORTED=0
FAILED=0
VERIFIED=0

echo -e "${BLUE}=== LCI Safe Porting Script ===${NC}"
echo "Source: $ORIGINAL"
echo "Target: $TARGET"
echo "Log: $LOG_FILE"
echo ""

# Function to log
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# Function to verify directory exists
verify_source() {
    local path="$1"
    if [[ ! -e "$ORIGINAL/$path" ]]; then
        echo -e "${RED}ERROR: Source path does not exist: $ORIGINAL/$path${NC}"
        return 1
    fi
    return 0
}

# Function to calculate checksum
checksum() {
    local file="$1"
    if [[ -f "$file" ]]; then
        md5sum "$file" | awk '{print $1}'
    else
        echo "DIR"
    fi
}

# Function to safely copy a file with verification
safe_copy_file() {
    local src="$1"
    local dst="$2"
    local update_imports="${3:-true}"

    log "Copying: $src -> $dst"

    # Verify source exists
    if [[ ! -f "$ORIGINAL/$src" ]]; then
        echo -e "${RED}  ✗ Source not found: $ORIGINAL/$src${NC}"
        ((FAILED++))
        return 1
    fi

    # Calculate source checksum
    local src_checksum=$(checksum "$ORIGINAL/$src")
    log "  Source checksum: $src_checksum"

    # Create destination directory
    local dst_dir=$(dirname "$dst")
    mkdir -p "$dst_dir"

    # Copy file
    cp "$ORIGINAL/$src" "$dst"

    # Verify copy
    local dst_checksum=$(checksum "$dst")
    if [[ "$src_checksum" != "$dst_checksum" ]]; then
        echo -e "${RED}  ✗ Checksum mismatch after copy!${NC}"
        echo -e "${RED}    Source: $src_checksum${NC}"
        echo -e "${RED}    Dest:   $dst_checksum${NC}"
        ((FAILED++))
        return 1
    fi

    log "  Copy verified: $dst_checksum"

    # Update import paths if requested
    if [[ "$update_imports" == "true" ]] && [[ "$dst" == *.go ]]; then
        log "  Updating import paths..."

        # Backup before modification
        cp "$dst" "$dst.backup"

        # Update imports
        sed -i 's|standardbeagle/lightning-code-index|github.com/standardbeagle/lci|g' "$dst"

        # Verify file is still valid Go
        if ! gofmt -l "$dst" >/dev/null 2>&1; then
            echo -e "${RED}  ✗ Invalid Go syntax after import update${NC}"
            mv "$dst.backup" "$dst"
            ((FAILED++))
            return 1
        fi

        # Format the file
        gofmt -w "$dst"

        # Remove backup
        rm "$dst.backup"

        local final_checksum=$(checksum "$dst")
        log "  Import paths updated: $final_checksum"
    fi

    echo -e "${GREEN}  ✓ Success${NC}"
    ((PORTED++))
    ((VERIFIED++))
    return 0
}

# Function to safely copy a directory with verification
safe_copy_dir() {
    local src="$1"
    local dst="$2"
    local update_imports="${3:-true}"

    log "Copying directory: $src -> $dst"

    # Verify source exists
    if [[ ! -d "$ORIGINAL/$src" ]]; then
        echo -e "${RED}  ✗ Source directory not found: $ORIGINAL/$src${NC}"
        ((FAILED++))
        return 1
    fi

    # Create manifest of files to copy
    local manifest=$(mktemp)
    find "$ORIGINAL/$src" -type f > "$manifest"

    local file_count=$(wc -l < "$manifest")
    log "  Files to copy: $file_count"

    # Copy each file
    local copied=0
    local failed=0
    while IFS= read -r src_file; do
        local rel_path="${src_file#$ORIGINAL/$src/}"
        local dst_file="$dst/$rel_path"

        if safe_copy_file "${src_file#$ORIGINAL/}" "$dst_file" "$update_imports"; then
            ((copied++))
        else
            ((failed++))
        fi
    done < "$manifest"

    rm "$manifest"

    if [[ $failed -gt 0 ]]; then
        echo -e "${RED}  ✗ Directory copy had $failed failures${NC}"
        return 1
    fi

    echo -e "${GREEN}  ✓ Directory copied: $copied files${NC}"
    return 0
}

# Function to run tests and verify
verify_tests() {
    local test_path="$1"

    log "Running tests: $test_path"

    if go test -v "$test_path" 2>&1 | tee -a "$LOG_FILE"; then
        echo -e "${GREEN}  ✓ Tests passed${NC}"
        return 0
    else
        echo -e "${RED}  ✗ Tests failed${NC}"
        return 1
    fi
}

# Function to show dry run
dry_run() {
    echo -e "${YELLOW}DRY RUN - No files will be modified${NC}"
    echo ""
    echo "Would port the following:"
    echo "  P0 Critical:"
    echo "    - tests/search-comparison/ (4 Go files + fixtures)"
    echo "    - testing/integration/ (16 Python files)"
    echo "    - docs/testing-strategy.md"
    echo "    - Makefile"
    echo ""
    echo "  P1 High:"
    echo "    - tests/benchmarks/ (3 Go files)"
    echo "    - real_projects/ (as git submodules)"
    echo ""
    echo "Import paths would be updated:"
    echo "  standardbeagle/lightning-code-index -> github.com/standardbeagle/lci"
    echo ""
}

# Main porting function
port_p0_critical() {
    log "=== Porting P0 Critical Items ==="

    # 1. Port search comparison tests
    echo -e "${BLUE}Porting search comparison tests...${NC}"
    if safe_copy_dir "tests/search-comparison" "tests/search-comparison" true; then
        verify_tests "./tests/search-comparison/..." || log "WARNING: Search comparison tests need attention"
    fi

    # 2. Port MCP integration tests (no import updates needed for Python)
    echo -e "${BLUE}Porting MCP integration tests...${NC}"
    safe_copy_dir "testing/integration" "testing/integration" false

    # 3. Port testing documentation
    echo -e "${BLUE}Porting testing documentation...${NC}"
    safe_copy_file "docs/testing-strategy.md" "docs/testing-strategy.md" false
    safe_copy_file "testing/README.md" "docs/testing-guide.md" false

    # 4. Port Makefile
    echo -e "${BLUE}Porting Makefile...${NC}"
    safe_copy_file "Makefile" "Makefile" false

    log "=== P0 Critical porting complete ==="
}

port_p1_high() {
    log "=== Porting P1 High Priority Items ==="

    # 1. Port performance benchmarks
    echo -e "${BLUE}Porting performance benchmarks...${NC}"
    if safe_copy_dir "tests/benchmarks" "tests/benchmarks" true; then
        verify_tests "./tests/benchmarks/..." || log "WARNING: Benchmark tests need attention"
    fi

    # 2. Port real_projects README (submodules added separately)
    echo -e "${BLUE}Porting real_projects documentation...${NC}"
    safe_copy_file "real_projects/README.md" "real_projects/README.md" false

    log "=== P1 High priority porting complete ==="
}

# Parse arguments
PHASE="p0"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --p0)
            PHASE="p0"
            shift
            ;;
        --p1)
            PHASE="p1"
            shift
            ;;
        --all)
            PHASE="all"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--dry-run] [--p0|--p1|--all]"
            exit 1
            ;;
    esac
done

# Execute
if [[ "$DRY_RUN" == "true" ]]; then
    dry_run
    exit 0
fi

# Verify source repository exists
if [[ ! -d "$ORIGINAL" ]]; then
    echo -e "${RED}ERROR: Source repository not found: $ORIGINAL${NC}"
    exit 1
fi

# Create scripts directory if needed
mkdir -p scripts

# Start porting
log "Starting safe porting process"

case $PHASE in
    p0)
        port_p0_critical
        ;;
    p1)
        port_p1_high
        ;;
    all)
        port_p0_critical
        port_p1_high
        ;;
esac

# Summary
echo ""
echo -e "${BLUE}=== Porting Summary ===${NC}"
echo -e "Files ported: ${GREEN}$PORTED${NC}"
echo -e "Verified: ${GREEN}$VERIFIED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo -e "Log file: $LOG_FILE"

if [[ $FAILED -gt 0 ]]; then
    echo -e "${RED}Some operations failed. Check log for details.${NC}"
    exit 1
else
    echo -e "${GREEN}All operations completed successfully!${NC}"
fi
