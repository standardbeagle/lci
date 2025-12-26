package indexing

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"

	"github.com/standardbeagle/lci/internal/config"
	"github.com/standardbeagle/lci/internal/debug"
)

// FileWatcher monitors the file system for changes and triggers incremental updates
type FileWatcher struct {
	watcher   *fsnotify.Watcher
	config    *config.Config
	debouncer *eventDebouncer
	scanner   *FileScanner
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Callbacks for handling file events
	onFileChanged func(path string, eventType FileEventType)
	onFileCreated func(path string)
	onFileRemoved func(path string)

	// Watch mode statistics
	eventsProcessed int64
	errorCount      int64
	lastEventTime   time.Time
	statsMu         sync.RWMutex

	// Progress tracking callback
	onBatchStart func(count int)
	onBatchEnd   func(count int, duration time.Duration)
}

// FileEventType represents the type of file system event
type FileEventType int

const (
	FileEventCreate FileEventType = iota
	FileEventWrite
	FileEventRemove
	FileEventRename
)

// NewFileWatcher creates a new file watcher
func NewFileWatcher(cfg *config.Config, scanner *FileScanner) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FileWatcher{
		watcher:   watcher,
		config:    cfg,
		debouncer: newEventDebouncer(time.Duration(cfg.Index.WatchDebounceMs) * time.Millisecond),
		scanner:   scanner,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Set up the debouncer callbacks
	fw.debouncer.setCallbacks(fw)

	return fw, nil
}

// SetCallbacks sets the callbacks for handling file events
func (fw *FileWatcher) SetCallbacks(
	onFileChanged func(path string, eventType FileEventType),
	onFileCreated func(path string),
	onFileRemoved func(path string),
) {
	fw.onFileChanged = onFileChanged
	fw.onFileCreated = onFileCreated
	fw.onFileRemoved = onFileRemoved
}

// SetProgressCallbacks sets callbacks for batch processing progress
func (fw *FileWatcher) SetProgressCallbacks(
	onBatchStart func(count int),
	onBatchEnd func(count int, duration time.Duration),
) {
	fw.onBatchStart = onBatchStart
	fw.onBatchEnd = onBatchEnd
}

// Start begins watching the configured directory
func (fw *FileWatcher) Start(root string) error {
	if !fw.config.Index.WatchMode {
		log.Printf("File watching disabled in configuration")
		return nil
	}

	debug.LogIndexing("Starting file watcher for directory: %s\n", root)

	// Add watches for all directories
	if err := fw.addWatches(root); err != nil {
		return fmt.Errorf("failed to add watches starting from %s: %w", root, err)
	}

	// Start the event processing goroutine
	fw.wg.Add(1)
	go fw.processEvents()

	// Start the debouncer
	fw.wg.Add(1)
	go fw.debouncer.run(fw.ctx, &fw.wg)

	debug.LogIndexing("File watcher started successfully\n")
	return nil
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() error {
	log.Printf("Stopping file watcher...")

	fw.cancel()

	// Close the fsnotify watcher
	if err := fw.watcher.Close(); err != nil {
		log.Printf("Error closing fsnotify watcher: %v", err)
	}

	// Wait for goroutines to finish
	fw.wg.Wait()

	log.Printf("File watcher stopped")
	return nil
}

// addWatches recursively adds watches to all relevant directories
func (fw *FileWatcher) addWatches(root string) error {
	// Track visited directories to prevent infinite loops from symlink cycles
	visitedDirs := make(map[string]bool)

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Check for symlink cycles - prevent infinite loops
		if info.IsDir() {
			// Get the real path to detect cycles
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil // Skip symlinks that can't be resolved
			}

			// Check if we've already visited this real directory
			if visitedDirs[realPath] {
				return filepath.SkipDir // Skip to prevent cycle
			}
			visitedDirs[realPath] = true
		}

		// Only watch directories
		if !info.IsDir() {
			return nil
		}

		// Skip directories that should be ignored
		if fw.shouldIgnoreDirectory(path, info) {
			return filepath.SkipDir
		}

		// Add watch for this directory
		if err := fw.watcher.Add(path); err != nil {
			log.Printf("Warning: failed to add watch for %s: %v", path, err)
			return nil // Continue despite errors
		}

		return nil
	})
}

// shouldIgnoreDirectory checks if a directory should be ignored based on configuration
func (fw *FileWatcher) shouldIgnoreDirectory(path string, info os.FileInfo) bool {
	// Check exclude patterns
	for _, pattern := range fw.config.Exclude {
		// Convert pattern to match directories
		dirPattern := pattern
		if strings.HasSuffix(pattern, "/**") {
			dirPattern = strings.TrimSuffix(pattern, "/**")
		}

		if matched, _ := filepath.Match(dirPattern, filepath.Base(path)); matched {
			return true
		}

		// Also check full path patterns
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	// Check gitignore if enabled
	if fw.scanner != nil && fw.scanner.gitignoreParser != nil {
		// Convert absolute path to relative path from project root for gitignore matching
		relativePath, err := filepath.Rel(fw.config.Project.Root, path)
		if err != nil {
			// If we can't get a relative path, log the error and use the original path
			// This is rare but can happen with unusual path configurations
			relativePath = path
		} else {
			// Use forward slashes for gitignore consistency
			relativePath = filepath.ToSlash(relativePath)
		}

		if fw.scanner.gitignoreParser.ShouldIgnore(relativePath, true) {
			return true
		}
	}

	return false
}

// processEvents processes file system events from fsnotify
func (fw *FileWatcher) processEvents() {
	defer fw.wg.Done()

	for {
		select {
		case <-fw.ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}

// handleEvent handles a single file system event
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	path := event.Name
	debug.LogIndexing("FileWatcher: received event %v for path %s\n", event.Op, path)

	// Check if this is a file we should process
	info, err := os.Stat(path)
	if err != nil {
		// File might have been deleted
		if event.Op&fsnotify.Remove != 0 {
			if fw.shouldProcessPath(path) {
				fw.debouncer.addEvent(path, FileEventRemove)
			}
		}
		return
	}

	// If it's a directory, handle directory events
	if info.IsDir() {
		fw.handleDirectoryEvent(event, path, info)
		return
	}

	// For files, enforce size limit early and filter by patterns
	if !info.IsDir() && info.Size() > int64(fw.config.Index.MaxFileSize) {
		debug.LogIndexing("FileWatcher: skipping oversized file %s (%d bytes > %d limit)\n", path, info.Size(), fw.config.Index.MaxFileSize)
		return
	}
	if !fw.shouldProcessPath(path) {
		debug.LogIndexing("FileWatcher: ignoring file %s (doesn't match patterns)\n", path)
		return
	}

	// Determine event type and add to debouncer
	var eventType FileEventType
	switch {
	case event.Op&fsnotify.Create != 0:
		eventType = FileEventCreate
	case event.Op&fsnotify.Write != 0:
		eventType = FileEventWrite
	case event.Op&fsnotify.Remove != 0:
		eventType = FileEventRemove
	case event.Op&fsnotify.Rename != 0:
		eventType = FileEventRename
	default:
		return // Ignore other events
	}

	debug.LogIndexing("FileWatcher: adding event %v for path %s to debouncer\n", eventType, path)
	fw.debouncer.addEvent(path, eventType)
}

// handleDirectoryEvent handles events for directories
func (fw *FileWatcher) handleDirectoryEvent(event fsnotify.Event, path string, info os.FileInfo) {
	// If a new directory was created, add a watch for it
	if event.Op&fsnotify.Create != 0 {
		if !fw.shouldIgnoreDirectory(path, info) {
			if err := fw.watcher.Add(path); err != nil {
				log.Printf("Warning: failed to add watch for new directory %s: %v", path, err)
			} else {
				log.Printf("Added watch for new directory: %s", path)
			}
		}
	}
}

// shouldProcessPath checks if a file path should be processed based on configuration
func (fw *FileWatcher) shouldProcessPath(path string) bool {
	// Create a fake FileInfo for directory check (we know it's not a directory here)
	info := &fakeFileInfo{name: filepath.Base(path), isDir: false}

	// Use the file scanner's logic to determine if we should process this file
	if fw.scanner != nil {
		return fw.scanner.shouldProcessFile(path, info)
	}

	// Fallback to basic checks using glob patterns
	for _, pattern := range fw.config.Include {
		// Use doublestar for glob pattern matching
		matched, err := doublestar.Match(pattern, path)
		if err == nil && matched {
			return true
		}

		// Also try matching against relative path from root
		if fw.config.Project.Root != "" {
			relPath, err := filepath.Rel(fw.config.Project.Root, path)
			if err == nil {
				if matched, _ := doublestar.Match(pattern, relPath); matched {
					return true
				}
			}
		}
	}

	return false
}

// eventDebouncer batches file events to avoid excessive processing
type eventDebouncer struct {
	events    map[string]FileEventType
	mutex     sync.Mutex
	debounce  time.Duration
	timer     *time.Timer
	callbacks *FileWatcher
}

// newEventDebouncer creates a new event debouncer
func newEventDebouncer(debounce time.Duration) *eventDebouncer {
	return &eventDebouncer{
		events:   make(map[string]FileEventType),
		debounce: debounce,
	}
}

// setCallbacks sets the callbacks reference for the debouncer
func (d *eventDebouncer) setCallbacks(fw *FileWatcher) {
	d.callbacks = fw
}

// addEvent adds a file event to be debounced
func (d *eventDebouncer) addEvent(path string, eventType FileEventType) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Store the latest event for this path
	d.events[path] = eventType

	// Reset the timer
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.debounce, d.flush)
}

// run starts the debouncer goroutine
func (d *eventDebouncer) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	<-ctx.Done()

	// DON'T flush on shutdown - it can deadlock with MasterIndex.Close()
	// The flush() calls onFileChanged() which tries to acquire mutexes that
	// may be held by the shutdown sequence. Events pending at shutdown are
	// acceptable to lose since the index is being torn down anyway.
}

// flush processes all accumulated events
func (d *eventDebouncer) flush() {
	d.mutex.Lock()
	events := d.events
	d.events = make(map[string]FileEventType)
	d.mutex.Unlock()

	if len(events) == 0 {
		return
	}

	log.Printf("Processing %d debounced file events", len(events))

	// Notify batch start if callback is set
	if d.callbacks != nil && d.callbacks.onBatchStart != nil {
		d.callbacks.onBatchStart(len(events))
	}

	batchStart := time.Now()

	// Group events by type for more efficient processing
	var creates, removes, changes []string
	for path, eventType := range events {
		switch eventType {
		case FileEventCreate:
			creates = append(creates, path)
		case FileEventRemove:
			removes = append(removes, path)
		case FileEventWrite, FileEventRename:
			changes = append(changes, path)
		}
	}

	// Process removals first (to free resources)
	for _, path := range removes {
		if d.callbacks != nil && d.callbacks.onFileRemoved != nil {
			d.callbacks.onFileRemoved(path)
			d.callbacks.incrementStats(1, 0)
		}
	}

	// Process changes
	for _, path := range changes {
		if d.callbacks != nil && d.callbacks.onFileChanged != nil {
			d.callbacks.onFileChanged(path, FileEventWrite)
			d.callbacks.incrementStats(1, 0)
		}
	}

	// Process creates last
	for _, path := range creates {
		if d.callbacks != nil && d.callbacks.onFileCreated != nil {
			d.callbacks.onFileCreated(path)
			d.callbacks.incrementStats(1, 0)
		}
	}

	// Notify batch end if callback is set
	if d.callbacks != nil && d.callbacks.onBatchEnd != nil {
		d.callbacks.onBatchEnd(len(events), time.Since(batchStart))
	}
}

// fakeFileInfo implements os.FileInfo for files we can't stat
type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f *fakeFileInfo) Name() string       { return f.name }
func (f *fakeFileInfo) Size() int64        { return 0 }
func (f *fakeFileInfo) Mode() os.FileMode  { return 0644 }
func (f *fakeFileInfo) ModTime() time.Time { return time.Now() }
func (f *fakeFileInfo) IsDir() bool        { return f.isDir }
func (f *fakeFileInfo) Sys() interface{}   { return nil }

// incrementStats updates watch mode statistics
func (fw *FileWatcher) incrementStats(events int64, errors int64) {
	fw.statsMu.Lock()
	defer fw.statsMu.Unlock()

	fw.eventsProcessed += events
	fw.errorCount += errors
	fw.lastEventTime = time.Now()
}

// GetStats returns current watch mode statistics
func (fw *FileWatcher) GetStats() WatchStats {
	fw.statsMu.RLock()
	defer fw.statsMu.RUnlock()

	return WatchStats{
		EventsProcessed: fw.eventsProcessed,
		ErrorCount:      fw.errorCount,
		LastEventTime:   fw.lastEventTime,
		IsActive:        fw.ctx.Err() == nil,
	}
}

// WatchStats contains statistics about file watching operations
type WatchStats struct {
	EventsProcessed int64
	ErrorCount      int64
	LastEventTime   time.Time
	IsActive        bool
}
