package indexing

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/debug"
	"github.com/standardbeagle/lci/internal/parser"
)

// NewFileScanner creates a new file scanner
func NewFileScanner(cfg *config.Config, bufferSize int) *FileScanner {
	scanner := &FileScanner{
		config:         cfg,
		bufferSize:     bufferSize,
		binaryDetector: NewBinaryDetector(),
	}

	// Pre-compile glob patterns for fast matching
	scanner.compilePatterns()

	// Initialize gitignore parser if enabled
	if cfg.Index.RespectGitignore {
		scanner.gitignoreParser = config.NewGitignoreParser()
		if err := scanner.gitignoreParser.LoadGitignore(cfg.Project.Root); err != nil {
			log.Printf("Warning: failed to load .gitignore: %v", err)
		}
	}

	return scanner
}

// CountFiles counts files that would be indexed without actually processing them.
// This uses the same exclusion/inclusion logic as ScanDirectory but only counts.
// Returns file count and total size in bytes.
func (fs *FileScanner) CountFiles(ctx context.Context, root string) (fileCount int, totalBytes int64, err error) {
	visitedDirs := make(map[string]bool)

	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if walkErr != nil {
			return nil // Continue despite errors
		}

		// Check for symlink cycles
		if info.IsDir() {
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil // Skip unresolvable symlinks
			}

			if visitedDirs[realPath] {
				return filepath.SkipDir // Prevent cycles
			}
			visitedDirs[realPath] = true
		}

		// Directory exclusion (same logic as ScanDirectory)
		if info.IsDir() && path != root {
			// Use relative path from root for pattern matching
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				relPath = path // Fallback to absolute path
			}
			normalizedPath := filepath.ToSlash(relPath)
			// Check with trailing slash for directory patterns
			if fs.shouldExcludeFast(normalizedPath) || fs.shouldExcludeFast(normalizedPath+"/") {
				return filepath.SkipDir
			}
			return nil
		}

		// File filtering (same logic as ScanDirectory)
		if !info.IsDir() {
			// Use relative path from root for pattern matching
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				relPath = path // Fallback to absolute path
			}
			normalizedPath := filepath.ToSlash(relPath)

			// Check exclusions (fast)
			if fs.shouldExcludeFast(normalizedPath) {
				return nil
			}

			// Check inclusions (fast)
			if !fs.shouldIncludeFast(normalizedPath) {
				return nil
			}

			// File matches - count it
			if fs.shouldProcessFile(path, info) {
				fileCount++
				totalBytes += info.Size()
			}
		}

		return nil
	})

	return fileCount, totalBytes, err
}

// ScanDirectory scans a directory and sends file tasks to the channel
func (fs *FileScanner) ScanDirectory(ctx context.Context, root string, taskChan chan<- FileTask, progress *ProgressTracker) error {
	var scannedFiles int64
	var processedFiles int64
	var lastMemCheck time.Time

	// Track visited directories to prevent infinite loops from symlink cycles
	visitedDirs := make(map[string]bool)

	// Memory monitoring - capture baseline for this scan
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	baselineMemMB := memStats.HeapAlloc / 1024 / 1024

	debug.LogIndexing("Starting directory scan of %s", root)

	// Single pass: find and process files with early directory pruning
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			debug.LogIndexing("Scanner error for %s: %v", path, err)
			return nil // Continue scanning despite errors
		}

		// Memory monitoring every 1000 files or 5 seconds
		currentFiles := atomic.LoadInt64(&scannedFiles)

		if currentFiles%1000 == 0 || time.Since(lastMemCheck) > 5*time.Second {
			runtime.ReadMemStats(&memStats)
			currentMemMB := memStats.HeapAlloc / 1024 / 1024

			// Calculate delta with protection against underflow (GC can reduce memory below baseline)
			var memDeltaMB uint64
			if currentMemMB > baselineMemMB {
				memDeltaMB = currentMemMB - baselineMemMB
			} else {
				memDeltaMB = 0 // GC reduced memory below baseline
			}

			debug.LogIndexing("Scanned %d files, Memory: %dMB (+%dMB from baseline), Visited dirs: %d, Current: %s",
				currentFiles, currentMemMB, memDeltaMB, len(visitedDirs), path)
			lastMemCheck = time.Now()

			// Emergency brake if memory delta for THIS scan is excessive
			// Limit per-scan memory usage, not global process memory
			// Use 2GB delta limit in tests (GO_TEST env var), 1GB otherwise
			memDeltaLimit := uint64(1000)
			if os.Getenv("GO_TEST") == "1" {
				memDeltaLimit = 2000
			}
			if memDeltaMB > memDeltaLimit {
				debug.LogIndexing("EMERGENCY: This scan used %dMB (limit: %dMB), aborting", memDeltaMB, memDeltaLimit)
				return fmt.Errorf("scan memory usage exceeded %dMB limit", memDeltaLimit)
			}
		}

		// Check for symlink cycles - prevent infinite loops
		if info.IsDir() {
			// Get the real path to detect cycles
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				debug.LogIndexing("Skipping unresolvable symlink: %s (error: %v)", path, err)
				return nil // Skip symlinks that can't be resolved
			}

			// Check if we've already visited this real directory
			if visitedDirs[realPath] {
				debug.LogIndexing("Cycle detected, skipping already visited: %s -> %s", path, realPath)
				return filepath.SkipDir // Skip to prevent cycle
			}
			visitedDirs[realPath] = true

			// Log directory entries to spot infinite loops
			if len(visitedDirs)%100 == 0 {
				debug.LogIndexing("Visited %d unique directories so far", len(visitedDirs))
			}
		}

		// Early directory pruning - skip entire excluded directories
		if info.IsDir() && path != root {
			// Use relative path from root for pattern matching
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				relPath = path // Fallback to absolute path
			}
			normalizedPath := filepath.ToSlash(relPath)
			// Check if this directory itself should be excluded
			if fs.shouldExcludeFast(normalizedPath) || fs.shouldExcludeFast(normalizedPath+"/") {
				return filepath.SkipDir
			}
			return nil // Continue into this directory
		}

		// For files, do quick filename filtering BEFORE expensive operations
		if !info.IsDir() {
			// Use relative path from root for pattern matching
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				relPath = path // Fallback to absolute path
			}
			normalizedPath := filepath.ToSlash(relPath)

			// Quick filename-based exclusion first (fast check)
			if fs.shouldExcludeFast(normalizedPath) {
				return nil // Skip this file
			}

			// Quick filename-based inclusion check (fast)
			if !fs.shouldIncludeFast(normalizedPath) {
				return nil // Skip this file - doesn't match any include pattern
			}
		}

		atomic.AddInt64(&scannedFiles, 1)

		if fs.shouldProcessFile(path, info) {
			atomic.AddInt64(&processedFiles, 1)

			// Detect language from file extension for parser selection
			ext := filepath.Ext(path)
			language := parser.GetLanguageFromExtension(ext)

			task := FileTask{
				Path:     path,
				Info:     info,
				Language: language,
				Priority: fs.getFilePriority(path),
			}

			// Send task with adaptive back-pressure instead of timeout failure
			select {
			case taskChan <- task:
				progress.IncrementScanned()
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(taskChannelTimeout):
				// Instead of failing, implement adaptive back-pressure
				debug.LogIndexing("Scanner: channel full at %s (processed %d files), implementing back-pressure", path, processedFiles)

				// Wait longer with exponential backoff but keep trying
				backoffDelay := taskChannelTimeout
				for retries := 0; retries < 5; retries++ {
					select {
					case taskChan <- task:
						progress.IncrementScanned()
						debug.LogIndexing("Scanner: successfully sent task for %s after %d retries", path, retries+1)
						goto taskSent
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(backoffDelay):
						debug.LogIndexing("Scanner: retry %d for %s (channel capacity: %d)", retries+1, path, cap(taskChan))
						backoffDelay *= 2 // Exponential backoff
					}
				}

				// If still can't send after retries, this indicates a serious pipeline issue
				return fmt.Errorf("scanner unable to send task for %s after 5 retries (total delay: %v): pipeline may be deadlocked",
					path, taskChannelTimeout*(1+2+4+8+16))
			}
		taskSent:
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking directory tree from %s (scanned %d files, processed %d): %w",
			root, scannedFiles, processedFiles, err)
	}

	// Only show scanner stats in debug mode
	debug.LogIndexing("File scanner: found %d files to process (scanned %d total files, root: %s)\n", processedFiles, scannedFiles, root)

	progress.SetTotal(int(processedFiles))
	return nil
}
