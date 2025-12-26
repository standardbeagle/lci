package errors

import (
	"fmt"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// Error types for the lightning-code-index system
type ErrorType string

const (
	// Indexing errors
	ErrorTypeIndexing ErrorType = "indexing"
	ErrorTypeParse    ErrorType = "parse"
	ErrorTypeSearch   ErrorType = "search"

	// File errors
	ErrorTypeFileNotFound ErrorType = "file_not_found"
	ErrorTypeFileTooLarge ErrorType = "file_too_large"
	ErrorTypePermission   ErrorType = "permission"

	// Configuration errors
	ErrorTypeConfig ErrorType = "config"

	// Internal errors
	ErrorTypeInternal ErrorType = "internal"
)

// IndexingError represents an error during the indexing process
type IndexingError struct {
	Type        ErrorType
	FileID      types.FileID
	FilePath    string
	Operation   string
	Underlying  error
	Timestamp   time.Time
	Recoverable bool
}

// NewIndexingError creates a new indexing error with context
func NewIndexingError(op string, err error) *IndexingError {
	return &IndexingError{
		Type:       ErrorTypeIndexing,
		Operation:  op,
		Underlying: err,
		Timestamp:  time.Now(),
	}
}

// WithFile adds file information to the error
func (e *IndexingError) WithFile(fileID types.FileID, path string) *IndexingError {
	e.FileID = fileID
	e.FilePath = path
	return e
}

// WithRecoverable marks the error as recoverable
func (e *IndexingError) WithRecoverable(recoverable bool) *IndexingError {
	e.Recoverable = recoverable
	return e
}

// Error implements the error interface
func (e *IndexingError) Error() string {
	if e.FilePath != "" {
		return fmt.Sprintf("%s %s failed for %s: %v", e.Type, e.Operation, e.FilePath, e.Underlying)
	}
	return fmt.Sprintf("%s %s failed: %v", e.Type, e.Operation, e.Underlying)
}

// Unwrap returns the underlying error for errors.Is/As
func (e *IndexingError) Unwrap() error {
	return e.Underlying
}

// IsRecoverable checks if the error can be retried
func (e *IndexingError) IsRecoverable() bool {
	return e.Recoverable
}

// ParseError represents a parsing error
type ParseError struct {
	Type       ErrorType
	FileID     types.FileID
	FilePath   string
	Line       int
	Column     int
	Token      string
	Underlying error
	Timestamp  time.Time
}

// NewParseError creates a new parse error
func NewParseError(fileID types.FileID, path string, line, column int, token string, err error) *ParseError {
	return &ParseError{
		Type:       ErrorTypeParse,
		FileID:     fileID,
		FilePath:   path,
		Line:       line,
		Column:     column,
		Token:      token,
		Underlying: err,
		Timestamp:  time.Now(),
	}
}

// Error implements the error interface
func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at %s:%d:%d (near token %q): %v",
		e.FilePath, e.Line, e.Column, e.Token, e.Underlying)
}

// Unwrap returns the underlying error
func (e *ParseError) Unwrap() error {
	return e.Underlying
}

// SearchError represents a search operation error
type SearchError struct {
	Type       ErrorType
	Pattern    string
	Underlying error
	Timestamp  time.Time
}

// NewSearchError creates a new search error
func NewSearchError(pattern string, err error) *SearchError {
	return &SearchError{
		Type:       ErrorTypeSearch,
		Pattern:    pattern,
		Underlying: err,
		Timestamp:  time.Now(),
	}
}

// Error implements the error interface
func (e *SearchError) Error() string {
	return fmt.Sprintf("search failed for pattern %q: %v", e.Pattern, e.Underlying)
}

// Unwrap returns the underlying error
func (e *SearchError) Unwrap() error {
	return e.Underlying
}

// FileError represents a file-related error
type FileError struct {
	Type       ErrorType
	Path       string
	Operation  string
	Underlying error
	Timestamp  time.Time
}

// NewFileError creates a new file error
func NewFileError(op, path string, err error) *FileError {
	errorType := ErrorTypeFileNotFound
	if isPermissionError(err) {
		errorType = ErrorTypePermission
	}

	return &FileError{
		Type:       errorType,
		Path:       path,
		Operation:  op,
		Underlying: err,
		Timestamp:  time.Now(),
	}
}

// isPermissionError checks if the error is a permission error
func isPermissionError(err error) bool {
	errStr := err.Error()
	return errStr == "permission denied" || errStr == "access denied"
}

// Error implements the error interface
func (e *FileError) Error() string {
	return fmt.Sprintf("file %s failed for %s: %v", e.Operation, e.Path, e.Underlying)
}

// Unwrap returns the underlying error
func (e *FileError) Unwrap() error {
	return e.Underlying
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field      string
	Value      string
	Underlying error
	Timestamp  time.Time
}

// NewConfigError creates a new config error
func NewConfigError(field, value string, err error) *ConfigError {
	return &ConfigError{
		Field:      field,
		Value:      value,
		Underlying: err,
		Timestamp:  time.Now(),
	}
}

// Error implements the error interface
func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error for field %s (value %s): %v", e.Field, e.Value, e.Underlying)
}

// Unwrap returns the underlying error
func (e *ConfigError) Unwrap() error {
	return e.Underlying
}

// MultiError represents multiple errors
type MultiError struct {
	Errors []error
}

// NewMultiError creates a new multi-error
func NewMultiError(errs []error) *MultiError {
	// Filter out nil errors
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return &MultiError{Errors: filtered}
}

// Error implements the error interface
func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%d errors: %v", len(e.Errors), e.Errors)
}

// Unwrap returns all errors
func (e *MultiError) Unwrap() []error {
	return e.Errors
}
