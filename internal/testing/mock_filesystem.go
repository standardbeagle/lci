package testing

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// MockFilesystem simulates filesystem operations for stress testing
type MockFilesystem struct {
	mu          sync.RWMutex
	files       map[string]*MockFile
	operations  []Operation
	errorMode   ErrorMode
	latencyMode LatencyMode
	changeSpeed time.Duration

	// Event tracking
	eventHistory []FileEvent
	eventMu      sync.Mutex
}

// MockFile represents a file in the mock filesystem
type MockFile struct {
	Path        string
	Content     []byte
	ModTime     time.Time
	Size        int64
	IsDirectory bool
	Exists      bool
	Version     int // Track file versions for change simulation
}

// Operation represents a filesystem operation
type Operation struct {
	Type      OpType
	Path      string
	Content   []byte
	Timestamp time.Time
	Error     error
}

// OpType represents the type of filesystem operation
type OpType int

const (
	OpCreate OpType = iota
	OpModify
	OpDelete
	OpMove
	OpRead
)

// ErrorMode controls error injection behavior
type ErrorMode int

const (
	ErrorNone ErrorMode = iota
	ErrorRandom
	ErrorSpecific
	ErrorBurst
)

// LatencyMode controls latency simulation
type LatencyMode int

const (
	LatencyNone LatencyMode = iota
	LatencyRandom
	LatencyFixed
	LatencyBurst
)

// FileEvent represents a file system event
type FileEvent struct {
	Type      EventType
	Path      string
	OldPath   string // For move events
	Timestamp time.Time
	Size      int64
}

// EventType represents the type of file event
type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
	EventMove
)

// NewMockFilesystem creates a new mock filesystem
func NewMockFilesystem() *MockFilesystem {
	return &MockFilesystem{
		files:       make(map[string]*MockFile),
		operations:  make([]Operation, 0),
		errorMode:   ErrorNone,
		latencyMode: LatencyNone,
		changeSpeed: 100 * time.Millisecond,
	}
}

// SetErrorMode configures error injection
func (mfs *MockFilesystem) SetErrorMode(mode ErrorMode) {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()
	mfs.errorMode = mode
}

// SetLatencyMode configures latency simulation
func (mfs *MockFilesystem) SetLatencyMode(mode LatencyMode) {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()
	mfs.latencyMode = mode
}

// SetChangeSpeed sets the speed of file changes
func (mfs *MockFilesystem) SetChangeSpeed(speed time.Duration) {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()
	mfs.changeSpeed = speed
}

// CreateFile creates a new file in the mock filesystem
func (mfs *MockFilesystem) CreateFile(path string, content []byte) error {
	if err := mfs.simulateError(); err != nil {
		return err
	}

	mfs.simulateLatency()

	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	file := &MockFile{
		Path:        path,
		Content:     make([]byte, len(content)),
		ModTime:     time.Now(),
		Size:        int64(len(content)),
		IsDirectory: false,
		Exists:      true,
		Version:     1,
	}
	copy(file.Content, content)

	mfs.files[path] = file
	mfs.recordOperation(OpCreate, path, content, nil)
	mfs.recordEvent(EventCreate, path, "", int64(len(content)))

	return nil
}

// ModifyFile modifies an existing file
func (mfs *MockFilesystem) ModifyFile(path string, content []byte) error {
	if err := mfs.simulateError(); err != nil {
		return err
	}

	mfs.simulateLatency()

	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	file, exists := mfs.files[path]
	if !exists || !file.Exists {
		return fmt.Errorf("file does not exist: %s", path)
	}

	file.Content = make([]byte, len(content))
	copy(file.Content, content)
	file.ModTime = time.Now()
	file.Size = int64(len(content))
	file.Version++

	mfs.recordOperation(OpModify, path, content, nil)
	mfs.recordEvent(EventModify, path, "", int64(len(content)))

	return nil
}

// DeleteFile deletes a file from the mock filesystem
func (mfs *MockFilesystem) DeleteFile(path string) error {
	if err := mfs.simulateError(); err != nil {
		return err
	}

	mfs.simulateLatency()

	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	file, exists := mfs.files[path]
	if !exists || !file.Exists {
		return fmt.Errorf("file does not exist: %s", path)
	}

	file.Exists = false
	mfs.recordOperation(OpDelete, path, nil, nil)
	mfs.recordEvent(EventDelete, path, "", file.Size)

	return nil
}

// ReadFile reads a file from the mock filesystem
func (mfs *MockFilesystem) ReadFile(path string) ([]byte, error) {
	if err := mfs.simulateError(); err != nil {
		return nil, err
	}

	mfs.simulateLatency()

	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	file, exists := mfs.files[path]
	if !exists || !file.Exists {
		return nil, fmt.Errorf("file does not exist: %s", path)
	}

	content := make([]byte, len(file.Content))
	copy(content, file.Content)

	mfs.recordOperation(OpRead, path, nil, nil)

	return content, nil
}

// StartSequentialWriter simulates a process writing sequential values
func (mfs *MockFilesystem) StartSequentialWriter(path string, count int, interval time.Duration) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := 0; i < count; i++ {
			content := fmt.Sprintf("value_%d\ncount=%d\ntimestamp=%d\n",
				i, i+1, time.Now().Unix())

			if i == 0 {
				_ = mfs.CreateFile(path, []byte(content))
			} else {
				_ = mfs.ModifyFile(path, []byte(content))
			}

			time.Sleep(interval)
		}
	}()

	return done
}

// StartRandomBitFlipper simulates random single character changes
func (mfs *MockFilesystem) StartRandomBitFlipper(path string, content []byte, flips int, interval time.Duration) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		// Create initial file
		_ = mfs.CreateFile(path, content)

		workingContent := make([]byte, len(content))
		copy(workingContent, content)

		for i := 0; i < flips; i++ {
			// Random position
			pos := rand.Intn(len(workingContent))

			// Random character (printable ASCII)
			workingContent[pos] = byte(32 + rand.Intn(95))

			_ = mfs.ModifyFile(path, workingContent)
			time.Sleep(interval)
		}
	}()

	return done
}

// StartCodeEvolver simulates realistic code changes
func (mfs *MockFilesystem) StartCodeEvolver(path string, baseContent []byte, changes int, interval time.Duration) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		// Create initial file
		_ = mfs.CreateFile(path, baseContent)

		content := string(baseContent)

		for i := 0; i < changes; i++ {
			// Simulate different types of code changes
			changeType := rand.Intn(4)

			switch changeType {
			case 0: // Add a line
				lines := splitLines(content)
				pos := rand.Intn(len(lines))
				newLine := fmt.Sprintf("    // Added line %d", i)
				lines = append(lines[:pos], append([]string{newLine}, lines[pos:]...)...)
				content = joinLines(lines)

			case 1: // Modify a line
				lines := splitLines(content)
				if len(lines) > 0 {
					pos := rand.Intn(len(lines))
					lines[pos] = lines[pos] + fmt.Sprintf(" // modified %d", i)
				}
				content = joinLines(lines)

			case 2: // Delete a line (if not too short)
				lines := splitLines(content)
				if len(lines) > 3 {
					pos := rand.Intn(len(lines))
					lines = append(lines[:pos], lines[pos+1:]...)
					content = joinLines(lines)
				}

			case 3: // Add function
				newFunc := fmt.Sprintf("\nfunc generated%d() {\n    return %d\n}\n", i, i)
				content = content + newFunc
			}

			_ = mfs.ModifyFile(path, []byte(content))
			time.Sleep(interval)
		}
	}()

	return done
}

// GetEventHistory returns the history of file events
func (mfs *MockFilesystem) GetEventHistory() []FileEvent {
	mfs.eventMu.Lock()
	defer mfs.eventMu.Unlock()

	history := make([]FileEvent, len(mfs.eventHistory))
	copy(history, mfs.eventHistory)
	return history
}

// GetOperationHistory returns the history of operations
func (mfs *MockFilesystem) GetOperationHistory() []Operation {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	history := make([]Operation, len(mfs.operations))
	copy(history, mfs.operations)
	return history
}

// ClearHistory clears operation and event history
func (mfs *MockFilesystem) ClearHistory() {
	mfs.mu.Lock()
	mfs.operations = mfs.operations[:0]
	mfs.mu.Unlock()

	mfs.eventMu.Lock()
	mfs.eventHistory = mfs.eventHistory[:0]
	mfs.eventMu.Unlock()
}

// Private helper methods

func (mfs *MockFilesystem) simulateError() error {
	switch mfs.errorMode {
	case ErrorRandom:
		if rand.Float32() < 0.1 { // 10% error rate
			return errors.New("simulated random error")
		}
	case ErrorSpecific:
		// Could be extended to target specific paths
		return nil
	case ErrorBurst:
		// Could implement burst error patterns
		return nil
	}
	return nil
}

func (mfs *MockFilesystem) simulateLatency() {
	switch mfs.latencyMode {
	case LatencyRandom:
		delay := time.Duration(rand.Intn(100)) * time.Millisecond
		time.Sleep(delay)
	case LatencyFixed:
		time.Sleep(50 * time.Millisecond)
	case LatencyBurst:
		if rand.Float32() < 0.1 { // 10% chance of high latency
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (mfs *MockFilesystem) recordOperation(opType OpType, path string, content []byte, err error) {
	op := Operation{
		Type:      opType,
		Path:      path,
		Content:   content,
		Timestamp: time.Now(),
		Error:     err,
	}
	mfs.operations = append(mfs.operations, op)
}

func (mfs *MockFilesystem) recordEvent(eventType EventType, path, oldPath string, size int64) {
	mfs.eventMu.Lock()
	defer mfs.eventMu.Unlock()

	event := FileEvent{
		Type:      eventType,
		Path:      path,
		OldPath:   oldPath,
		Timestamp: time.Now(),
		Size:      size,
	}
	mfs.eventHistory = append(mfs.eventHistory, event)
}

// Helper functions for code evolution

func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}

	var lines []string
	start := 0

	for i, char := range content {
		if char == '\n' {
			lines = append(lines, content[start:i])
			start = i + 1
		}
	}

	// Add remaining content
	if start < len(content) {
		lines = append(lines, content[start:])
	}

	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}

	return result
}
