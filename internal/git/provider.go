package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Provider wraps git commands to extract file states at different refs
type Provider struct {
	repoRoot string
}

// NewProvider creates a new git provider for the specified repository
func NewProvider(repoRoot string) (*Provider, error) {
	// Resolve to absolute path
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("invalid repo root: %w", err)
	}

	// Use git rev-parse --show-toplevel to find the actual repo root
	// This works from any subdirectory within a git repository
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %s (ensure you're in a git repository)", absRoot)
	}

	gitRoot := strings.TrimSpace(string(output))
	p := &Provider{repoRoot: gitRoot}

	return p, nil
}

// IsGitRepo checks if the directory is a git repository
func (p *Provider) IsGitRepo() bool {
	gitDir := filepath.Join(p.repoRoot, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetRepoRoot returns the repository root path
func (p *Provider) GetRepoRoot() string {
	return p.repoRoot
}

// GetChangedFiles returns the list of changed files based on analysis scope
func (p *Provider) GetChangedFiles(ctx context.Context, params AnalysisParams) ([]ChangedFile, error) {
	switch params.Scope {
	case ScopeStaged:
		return p.getStagedFiles(ctx)
	case ScopeWIP:
		return p.getWIPFiles(ctx)
	case ScopeCommit:
		return p.getCommitFiles(ctx, params.BaseRef)
	case ScopeRange:
		return p.getRangeFiles(ctx, params.BaseRef, params.TargetRef)
	default:
		return nil, fmt.Errorf("unknown scope: %s", params.Scope)
	}
}

// getStagedFiles returns files in the staging area
func (p *Provider) getStagedFiles(ctx context.Context) ([]ChangedFile, error) {
	// git diff --cached --name-status
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--name-status", "--no-renames")
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --cached failed: %w", err)
	}

	return p.parseNameStatus(output)
}

// getWIPFiles returns all uncommitted files (staged + unstaged)
func (p *Provider) getWIPFiles(ctx context.Context) ([]ChangedFile, error) {
	// git diff HEAD --name-status
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--name-status", "--no-renames")
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		// If HEAD doesn't exist (new repo), fall back to staged
		return p.getStagedFiles(ctx)
	}

	return p.parseNameStatus(output)
}

// getCommitFiles returns files changed in a specific commit
func (p *Provider) getCommitFiles(ctx context.Context, commitRef string) ([]ChangedFile, error) {
	if commitRef == "" {
		commitRef = "HEAD"
	}

	// git diff-tree --no-commit-id --name-status -r <commit>
	cmd := exec.CommandContext(ctx, "git", "diff-tree", "--no-commit-id", "--name-status", "-r", commitRef)
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree failed for %s: %w", commitRef, err)
	}

	return p.parseNameStatus(output)
}

// getRangeFiles returns files changed in a commit range
func (p *Provider) getRangeFiles(ctx context.Context, baseRef, targetRef string) ([]ChangedFile, error) {
	if baseRef == "" {
		return nil, errors.New("base_ref required for range scope")
	}
	if targetRef == "" {
		targetRef = "HEAD"
	}

	// git diff --name-status base..target
	rangeSpec := fmt.Sprintf("%s..%s", baseRef, targetRef)
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-status", "--no-renames", rangeSpec)
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s failed: %w", rangeSpec, err)
	}

	return p.parseNameStatus(output)
}

// parseNameStatus parses git name-status output
func (p *Provider) parseNameStatus(output []byte) ([]ChangedFile, error) {
	var files []ChangedFile

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		path := parts[1]
		oldPath := ""

		// Handle rename/copy with old path
		if len(parts) >= 3 && (status[0] == 'R' || status[0] == 'C') {
			oldPath = parts[1]
			path = parts[2]
		}

		file := ChangedFile{
			Path:    path,
			OldPath: oldPath,
			Status:  p.parseStatus(status),
		}

		files = append(files, file)
	}

	return files, scanner.Err()
}

// parseStatus converts git status letter to FileChangeStatus
func (p *Provider) parseStatus(status string) FileChangeStatus {
	if len(status) == 0 {
		return FileStatusModified
	}

	switch status[0] {
	case 'A':
		return FileStatusAdded
	case 'D':
		return FileStatusDeleted
	case 'M':
		return FileStatusModified
	case 'R':
		return FileStatusRenamed
	case 'C':
		return FileStatusCopied
	default:
		return FileStatusModified
	}
}

// GetFileContent returns the content of a file at a specific ref
// If ref is empty, returns the working tree version
func (p *Provider) GetFileContent(ctx context.Context, ref, path string) ([]byte, error) {
	if ref == "" || ref == "WORKING" {
		// Return working tree content
		fullPath := filepath.Join(p.repoRoot, path)
		return os.ReadFile(fullPath)
	}

	if ref == "STAGED" || ref == "INDEX" {
		// Return staged content
		return p.getStagedContent(ctx, path)
	}

	// git show ref:path
	spec := fmt.Sprintf("%s:%s", ref, path)
	cmd := exec.CommandContext(ctx, "git", "show", spec)
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s failed: %w", spec, err)
	}

	return output, nil
}

// getStagedContent returns the staged content of a file
func (p *Provider) getStagedContent(ctx context.Context, path string) ([]byte, error) {
	// git show :path (colon prefix means index/staged)
	spec := ":" + path
	cmd := exec.CommandContext(ctx, "git", "show", spec)
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s failed: %w", spec, err)
	}

	return output, nil
}

// GetDiffStats returns statistics about the diff
func (p *Provider) GetDiffStats(ctx context.Context, params AnalysisParams) (DiffStats, error) {
	var args []string

	switch params.Scope {
	case ScopeStaged:
		args = []string{"diff", "--cached", "--numstat"}
	case ScopeWIP:
		args = []string{"diff", "HEAD", "--numstat"}
	case ScopeCommit:
		ref := params.BaseRef
		if ref == "" {
			ref = "HEAD"
		}
		args = []string{"diff-tree", "--no-commit-id", "--numstat", "-r", ref}
	case ScopeRange:
		if params.BaseRef == "" {
			return DiffStats{}, errors.New("base_ref required for range scope")
		}
		target := params.TargetRef
		if target == "" {
			target = "HEAD"
		}
		args = []string{"diff", "--numstat", fmt.Sprintf("%s..%s", params.BaseRef, target)}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return DiffStats{}, fmt.Errorf("git diff --numstat failed: %w", err)
	}

	return p.parseNumstat(output)
}

// parseNumstat parses git --numstat output
func (p *Provider) parseNumstat(output []byte) (DiffStats, error) {
	var stats DiffStats

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])

		stats.TotalAdded += added
		stats.TotalDeleted += deleted

		// Determine file change type based on add/delete counts
		if added > 0 && deleted == 0 {
			stats.FilesAdded++
		} else if added == 0 && deleted > 0 {
			stats.FilesDeleted++
		} else {
			stats.FilesModified++
		}
	}

	return stats, scanner.Err()
}

// GetBaseRef determines the appropriate base reference for a scope
func (p *Provider) GetBaseRef(ctx context.Context, params AnalysisParams) (string, error) {
	switch params.Scope {
	case ScopeStaged, ScopeWIP:
		return "HEAD", nil
	case ScopeCommit:
		ref := params.BaseRef
		if ref == "" {
			ref = "HEAD"
		}
		// Get parent of the commit
		return p.getParentCommit(ctx, ref)
	case ScopeRange:
		if params.BaseRef == "" {
			return "", errors.New("base_ref required for range scope")
		}
		return params.BaseRef, nil
	default:
		return "HEAD", nil
	}
}

// getParentCommit returns the parent commit of a given commit
func (p *Provider) getParentCommit(ctx context.Context, commit string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", commit+"^")
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		// No parent (first commit), return empty tree
		return "4b825dc642cb6eb9a060e54bf8d69288fbee4904", nil // Empty tree hash
	}

	return strings.TrimSpace(string(output)), nil
}

// GetTargetRef determines the appropriate target reference for a scope
func (p *Provider) GetTargetRef(params AnalysisParams) string {
	switch params.Scope {
	case ScopeStaged:
		return "STAGED"
	case ScopeWIP:
		return "WORKING"
	case ScopeCommit:
		if params.BaseRef != "" {
			return params.BaseRef
		}
		return "HEAD"
	case ScopeRange:
		if params.TargetRef != "" {
			return params.TargetRef
		}
		return "HEAD"
	default:
		return "HEAD"
	}
}

// ListAllFiles returns all tracked files in the repository
func (p *Provider) ListAllFiles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files")
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files failed: %w", err)
	}

	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		file := scanner.Text()
		if file != "" {
			files = append(files, file)
		}
	}

	return files, scanner.Err()
}

// GetCurrentBranch returns the current branch name
func (p *Provider) GetCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCommitHash returns the full commit hash for a reference
func (p *Provider) GetCommitHash(ctx context.Context, ref string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", ref)
	cmd.Dir = p.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s failed: %w", ref, err)
	}

	return strings.TrimSpace(string(output)), nil
}
