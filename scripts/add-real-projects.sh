#!/bin/bash
# Safely add real_projects as git submodules
# Prevents copy errors by using git submodules directly

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

LOG_FILE="add-real-projects-$(date +%Y%m%d-%H%M%S).log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

echo -e "${BLUE}=== Adding real_projects as git submodules ===${NC}"
echo "Log: $LOG_FILE"
echo ""

# Array of projects: "lang:name:url"
PROJECTS=(
    "go:chi:https://github.com/go-chi/chi.git"
    "go:go-github:https://github.com/google/go-github.git"
    "go:pocketbase:https://github.com/pocketbase/pocketbase.git"
    "python:fastapi:https://github.com/tiangolo/fastapi.git"
    "python:httpx:https://github.com/encode/httpx.git"
    "python:pydantic:https://github.com/pydantic/pydantic.git"
    "typescript:next.js:https://github.com/vercel/next.js.git"
    "typescript:shadcn-ui:https://github.com/shadcn-ui/ui.git"
    "typescript:trpc:https://github.com/trpc/trpc.git"
)

# Minimal projects for initial setup
MINIMAL_PROJECTS=(
    "go:chi:https://github.com/go-chi/chi.git"
    "python:fastapi:https://github.com/tiangolo/fastapi.git"
)

MODE="minimal"
DRY_RUN=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --full)
            MODE="full"
            shift
            ;;
        --minimal)
            MODE="minimal"
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--minimal|--full] [--dry-run]"
            exit 1
            ;;
    esac
done

# Select projects based on mode
if [[ "$MODE" == "full" ]]; then
    SELECTED_PROJECTS=("${PROJECTS[@]}")
    echo -e "${BLUE}Mode: Full (9 projects)${NC}"
else
    SELECTED_PROJECTS=("${MINIMAL_PROJECTS[@]}")
    echo -e "${BLUE}Mode: Minimal (2 projects)${NC}"
fi

if [[ "$DRY_RUN" == "true" ]]; then
    echo -e "${YELLOW}DRY RUN - No changes will be made${NC}"
    echo ""
    echo "Would add the following submodules:"
    for project in "${SELECTED_PROJECTS[@]}"; do
        IFS=':' read -r lang name url <<< "$project"
        echo "  - real_projects/$lang/$name ($url)"
    done
    exit 0
fi

# Create real_projects directories
log "Creating directory structure..."
mkdir -p real_projects/{go,python,typescript}

ADDED=0
FAILED=0

# Add each submodule
for project in "${SELECTED_PROJECTS[@]}"; do
    IFS=':' read -r lang name url <<< "$project"
    path="real_projects/$lang/$name"

    log "Adding: $path"
    echo -e "${BLUE}Adding submodule: $path${NC}"

    # Check if submodule already exists
    if [[ -d "$path/.git" ]] || git config --file .gitmodules "submodule.$path.url" &>/dev/null; then
        echo -e "${YELLOW}  ⚠ Already exists, skipping${NC}"
        log "  Skipped (already exists)"
        continue
    fi

    # Add submodule
    if git submodule add "$url" "$path" 2>&1 | tee -a "$LOG_FILE"; then
        echo -e "${GREEN}  ✓ Added${NC}"
        log "  Success"
        ((ADDED++))

        # Initialize and update
        if git submodule update --init --recursive "$path" 2>&1 | tee -a "$LOG_FILE"; then
            echo -e "${GREEN}  ✓ Initialized${NC}"
            log "  Initialized"

            # Verify checkout
            if [[ -f "$path/README.md" ]] || [[ -f "$path/readme.md" ]]; then
                echo -e "${GREEN}  ✓ Verified (README found)${NC}"
                log "  Verified"
            else
                echo -e "${YELLOW}  ⚠ No README found (may be OK)${NC}"
                log "  Warning: No README"
            fi
        else
            echo -e "${RED}  ✗ Failed to initialize${NC}"
            log "  Error: Failed to initialize"
            ((FAILED++))
        fi
    else
        echo -e "${RED}  ✗ Failed to add submodule${NC}"
        log "  Error: Failed to add"
        ((FAILED++))
    fi

    echo ""
done

# Copy README from original if it exists
ORIGINAL="../lightning-code-index"
if [[ -f "$ORIGINAL/real_projects/README.md" ]]; then
    log "Copying real_projects README..."
    cp "$ORIGINAL/real_projects/README.md" real_projects/README.md
    echo -e "${GREEN}✓ README copied${NC}"
fi

# Update .lci.kdl to exclude real_projects from indexing
log "Updating .lci.kdl to exclude real_projects..."
if [[ -f ".lci.kdl" ]]; then
    if ! grep -q "real_projects" .lci.kdl; then
        echo 'exclude "real_projects/**"' >> .lci.kdl
        echo -e "${GREEN}✓ Exclusion added to .lci.kdl${NC}"
    else
        echo -e "${YELLOW}⚠ real_projects already excluded in .lci.kdl${NC}"
    fi
else
    echo -e "${YELLOW}⚠ No .lci.kdl found, create one to exclude real_projects from indexing${NC}"
fi

# Summary
echo ""
echo -e "${BLUE}=== Summary ===${NC}"
echo -e "Submodules added: ${GREEN}$ADDED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo -e "Log file: $LOG_FILE"

if [[ $FAILED -gt 0 ]]; then
    echo -e "${RED}Some operations failed. Check log for details.${NC}"
    exit 1
else
    echo -e "${GREEN}All operations completed successfully!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Review changes: git status"
    echo "  2. Run tests with real projects"
    echo "  3. Commit: git commit -m 'Add real_projects test infrastructure'"
fi
