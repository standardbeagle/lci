package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// HistoryProvider extends Provider with change frequency analysis methods
type HistoryProvider struct {
	*Provider
}

// NewHistoryProvider creates a HistoryProvider from an existing Provider
func NewHistoryProvider(p *Provider) *HistoryProvider {
	return &HistoryProvider{Provider: p}
}

// GetCommitHistory returns commits within a time window
// Uses: git log --numstat --format="%H|%an|%ae|%at" --since="<time>"
func (h *HistoryProvider) GetCommitHistory(ctx context.Context, since time.Time, paths ...string) ([]CommitInfo, error) {
	sinceStr := since.Format("2006-01-02T15:04:05")

	args := []string{
		"log",
		"--numstat",
		"--format=%H|%an|%ae|%at|%s",
		"--since=" + sinceStr,
		"--no-merges",
	}

	// Add path filters if specified
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = h.repoRoot

	output, err := cmd.Output()
	if err != nil {
		// Check if it's just an empty result (no commits in range)
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	return h.parseCommitHistory(output)
}

// parseCommitHistory parses the custom git log output format
func (h *HistoryProvider) parseCommitHistory(output []byte) ([]CommitInfo, error) {
	var commits []CommitInfo
	var current *CommitInfo

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Check if this is a commit header line
		if strings.Contains(line, "|") && len(strings.Split(line, "|")) >= 4 {
			parts := strings.SplitN(line, "|", 5)
			if len(parts) >= 4 && len(parts[0]) >= 40 {
				// Save previous commit if exists
				if current != nil {
					commits = append(commits, *current)
				}

				timestamp, _ := strconv.ParseInt(parts[3], 10, 64)
				current = &CommitInfo{
					Hash:        parts[0],
					AuthorName:  parts[1],
					AuthorEmail: parts[2],
					Timestamp:   time.Unix(timestamp, 0),
				}
				if len(parts) >= 5 {
					current.Message = parts[4]
				}
				continue
			}
		}

		// Parse numstat line (added, deleted, path)
		if current != nil && line != "" {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				added, _ := strconv.Atoi(parts[0])
				deleted, _ := strconv.Atoi(parts[1])
				path := parts[2]

				// Handle renamed files
				oldPath := ""
				if strings.Contains(path, " => ") {
					// Format: old => new or {old => new}
					path, oldPath = parseRenamePath(path)
				}

				fc := FileChange{
					Path:         path,
					OldPath:      oldPath,
					LinesAdded:   added,
					LinesDeleted: deleted,
					Status:       determineStatus(added, deleted, oldPath),
				}
				current.FileChanges = append(current.FileChanges, fc)
			}
		}
	}

	// Don't forget the last commit
	if current != nil {
		commits = append(commits, *current)
	}

	return commits, scanner.Err()
}

// parseRenamePath handles git's rename notation
func parseRenamePath(path string) (newPath, oldPath string) {
	// Handle {prefix/old => prefix/new} format
	if strings.Contains(path, "{") {
		// More complex rename format
		re := regexp.MustCompile(`(.*)\{(.*) => (.*)\}(.*)`)
		if matches := re.FindStringSubmatch(path); len(matches) == 5 {
			prefix := matches[1]
			oldPart := matches[2]
			newPart := matches[3]
			suffix := matches[4]
			return prefix + newPart + suffix, prefix + oldPart + suffix
		}
	}

	// Simple old => new format
	parts := strings.Split(path, " => ")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
	}

	return path, ""
}

// determineStatus determines the change status from stats
func determineStatus(added, deleted int, oldPath string) string {
	if oldPath != "" {
		return "R" // Renamed
	}
	if added > 0 && deleted == 0 {
		return "A" // Added
	}
	if added == 0 && deleted > 0 {
		return "D" // Deleted
	}
	return "M" // Modified
}

// GetFileHistory returns commit history for a specific file
func (h *HistoryProvider) GetFileHistory(ctx context.Context, filePath string, since time.Time) ([]CommitInfo, error) {
	return h.GetCommitHistory(ctx, since, filePath)
}

// GetSymbolHistory returns commit history for a specific line range (symbol)
// Uses: git log -L <start>,<end>:<path> --format="%H|%an|%ae|%at"
// Note: This is more expensive than file-level history
func (h *HistoryProvider) GetSymbolHistory(ctx context.Context, filePath string, startLine, endLine int, since time.Time) ([]CommitInfo, error) {
	sinceStr := since.Format("2006-01-02T15:04:05")
	lineRange := fmt.Sprintf("%d,%d:%s", startLine, endLine, filePath)

	args := []string{
		"log",
		"-L", lineRange,
		"--format=%H|%an|%ae|%at|%s",
		"--since=" + sinceStr,
		"--no-merges",
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = h.repoRoot

	output, err := cmd.Output()
	if err != nil {
		// Check for common errors
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "has only") || strings.Contains(stderr, "does not have") {
				// Line range doesn't exist in history
				return nil, nil
			}
		}
		return nil, fmt.Errorf("git log -L failed: %w", err)
	}

	return h.parseSymbolHistory(output)
}

// parseSymbolHistory parses git log -L output
func (h *HistoryProvider) parseSymbolHistory(output []byte) ([]CommitInfo, error) {
	var commits []CommitInfo
	var current *CommitInfo

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Check if this is a commit header line
		if strings.Contains(line, "|") && len(strings.Split(line, "|")) >= 4 {
			parts := strings.SplitN(line, "|", 5)
			if len(parts) >= 4 && len(parts[0]) >= 40 {
				// Save previous commit if exists
				if current != nil {
					commits = append(commits, *current)
				}

				timestamp, _ := strconv.ParseInt(parts[3], 10, 64)
				current = &CommitInfo{
					Hash:        parts[0],
					AuthorName:  parts[1],
					AuthorEmail: parts[2],
					Timestamp:   time.Unix(timestamp, 0),
				}
				if len(parts) >= 5 {
					current.Message = parts[4]
				}
			}
		}
		// git log -L includes diff output which we skip
		// The commit metadata is what we need
	}

	// Don't forget the last commit
	if current != nil {
		commits = append(commits, *current)
	}

	return commits, scanner.Err()
}

// GetFileContributors returns contributor statistics for a file
// Uses: git shortlog -sne -- <path>
func (h *HistoryProvider) GetFileContributors(ctx context.Context, filePath string, since time.Time) ([]ContributorActivity, error) {
	sinceStr := since.Format("2006-01-02T15:04:05")

	args := []string{
		"shortlog",
		"-sne",
		"--since=" + sinceStr,
		"--no-merges",
		"--",
		filePath,
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = h.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git shortlog failed: %w", err)
	}

	return h.parseShortlog(output)
}

// parseShortlog parses git shortlog -sne output
func (h *HistoryProvider) parseShortlog(output []byte) ([]ContributorActivity, error) {
	var contributors []ContributorActivity
	totalChanges := 0

	// First pass: collect data and total
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: "  123\tAuthor Name <email@example.com>"
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}

		count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}

		nameEmail := parts[1]
		name, email := parseNameEmail(nameEmail)

		contributors = append(contributors, ContributorActivity{
			AuthorName:  name,
			AuthorEmail: email,
			ChangeCount: count,
		})
		totalChanges += count
	}

	// Second pass: calculate ownership shares
	for i := range contributors {
		if totalChanges > 0 {
			contributors[i].OwnershipShare = float64(contributors[i].ChangeCount) / float64(totalChanges)
		}
	}

	// Sort by change count descending
	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i].ChangeCount > contributors[j].ChangeCount
	})

	return contributors, scanner.Err()
}

// parseNameEmail extracts name and email from "Name <email>" format
func parseNameEmail(s string) (name, email string) {
	s = strings.TrimSpace(s)
	if idx := strings.LastIndex(s, " <"); idx != -1 {
		name = s[:idx]
		email = strings.Trim(s[idx+2:], "<>")
	} else {
		name = s
	}
	return
}

// AggregateFileStats aggregates commit data into FileChangeFrequency
func (h *HistoryProvider) AggregateFileStats(commits []CommitInfo, filePath string, window TimeWindow) *FileChangeFrequency {
	if len(commits) == 0 {
		return nil
	}

	windowDuration := TimeWindowToDuration(window)
	windowDays := windowDuration.Hours() / 24

	metrics := &FrequencyMetrics{}
	authorStats := make(map[string]*ContributorActivity)

	for _, commit := range commits {
		metrics.ChangeCount++

		// Update first/last change times
		if metrics.FirstChangeAt.IsZero() || commit.Timestamp.Before(metrics.FirstChangeAt) {
			metrics.FirstChangeAt = commit.Timestamp
		}
		if commit.Timestamp.After(metrics.LastChangeAt) {
			metrics.LastChangeAt = commit.Timestamp
		}

		// Aggregate per-file stats
		for _, fc := range commit.FileChanges {
			if fc.Path == filePath || matchesPath(fc.Path, filePath) {
				metrics.LinesAdded += fc.LinesAdded
				metrics.LinesDeleted += fc.LinesDeleted
			}
		}

		// Track contributor
		key := commit.AuthorEmail
		if _, exists := authorStats[key]; !exists {
			authorStats[key] = &ContributorActivity{
				AuthorName:  commit.AuthorName,
				AuthorEmail: commit.AuthorEmail,
			}
		}
		authorStats[key].ChangeCount++
		if commit.Timestamp.After(authorStats[key].LastChangeAt) {
			authorStats[key].LastChangeAt = commit.Timestamp
		}
	}

	// Calculate derived metrics
	metrics.UniqueAuthors = len(authorStats)
	metrics.ChangeRate = float64(metrics.ChangeCount) / windowDays
	metrics.VolatilityScore = CalculateVolatilityScore(
		metrics.ChangeCount,
		metrics.LinesAdded+metrics.LinesDeleted,
		metrics.UniqueAuthors,
		windowDays,
	)

	// Build contributor list
	var contributors []ContributorActivity
	totalChanges := metrics.ChangeCount
	for _, ca := range authorStats {
		ca.OwnershipShare = float64(ca.ChangeCount) / float64(totalChanges)
		contributors = append(contributors, *ca)
	}

	// Sort by change count descending
	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i].ChangeCount > contributors[j].ChangeCount
	})

	return &FileChangeFrequency{
		FilePath: filePath,
		Metrics: map[TimeWindow]*FrequencyMetrics{
			window: metrics,
		},
		Contributors: contributors,
	}
}

// matchesPath checks if two paths refer to the same file
func matchesPath(p1, p2 string) bool {
	// Normalize paths
	p1 = filepath.Clean(p1)
	p2 = filepath.Clean(p2)

	// Direct match
	if p1 == p2 {
		return true
	}

	// Match just the filename if paths are partial
	return filepath.Base(p1) == filepath.Base(p2)
}

// GetRepoHistory returns aggregate history for the entire repository
func (h *HistoryProvider) GetRepoHistory(ctx context.Context, since time.Time, pattern string) ([]CommitInfo, error) {
	args := []string{
		"log",
		"--numstat",
		"--format=%H|%an|%ae|%at|%s",
		"--since=" + since.Format("2006-01-02T15:04:05"),
		"--no-merges",
	}

	// Add path pattern if specified
	if pattern != "" {
		// Expand glob pattern to matching files
		files, err := h.expandGlobPattern(ctx, pattern)
		if err != nil {
			return nil, err
		}
		if len(files) > 0 {
			args = append(args, "--")
			args = append(args, files...)
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = h.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	return h.parseCommitHistory(output)
}

// expandGlobPattern expands a glob pattern to matching tracked files
func (h *HistoryProvider) expandGlobPattern(ctx context.Context, pattern string) ([]string, error) {
	// Use git ls-files with pattern
	cmd := exec.CommandContext(ctx, "git", "ls-files", pattern)
	cmd.Dir = h.repoRoot

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

// GetCurrentAuthor returns the current git user
func (h *HistoryProvider) GetCurrentAuthor(ctx context.Context) (name, email string, err error) {
	// Get user name
	nameCmd := exec.CommandContext(ctx, "git", "config", "user.name")
	nameCmd.Dir = h.repoRoot
	nameOutput, err := nameCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("git config user.name failed: %w", err)
	}
	name = strings.TrimSpace(string(nameOutput))

	// Get user email
	emailCmd := exec.CommandContext(ctx, "git", "config", "user.email")
	emailCmd.Dir = h.repoRoot
	emailOutput, err := emailCmd.Output()
	if err != nil {
		return name, "", fmt.Errorf("git config user.email failed: %w", err)
	}
	email = strings.TrimSpace(string(emailOutput))

	return name, email, nil
}

// GetRecentCommitCount returns the number of commits in a time window
func (h *HistoryProvider) GetRecentCommitCount(ctx context.Context, filePath string, window time.Duration) (int, error) {
	since := time.Now().Add(-window)
	sinceStr := since.Format("2006-01-02T15:04:05")

	args := []string{
		"rev-list",
		"--count",
		"--since=" + sinceStr,
		"HEAD",
		"--",
		filePath,
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = h.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git rev-list --count failed: %w", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, fmt.Errorf("invalid count: %w", err)
	}

	return count, nil
}
