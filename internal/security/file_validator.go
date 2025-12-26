package security

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileValidator validates large files before loading them fully
// Prevents security issues and memory bloat from malicious files

type FileValidator struct {
	ValidationThreshold int64 // Files larger than this are validated first
	HeaderSize          int64 // Size of header to read for validation
}

func NewFileValidator(thresholdKB int64) *FileValidator {
	return &FileValidator{
		ValidationThreshold: thresholdKB * 1024,
		HeaderSize:          64 * 1024, // 64KB header
	}
}

// ValidateLargeFile reads only the header and validates the file is legitimate
// Returns error if file is invalid/malicious (image saved as code, binary data, etc.)
func (fv *FileValidator) ValidateLargeFile(path string) error {
	// Get file size
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Skip validation for small files
	if info.Size() <= fv.ValidationThreshold {
		return nil
	}

	// Read only header (64KB)
	header := make([]byte, fv.HeaderSize)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	n, err := io.ReadFull(f, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("failed to read header: %w", err)
	}
	header = header[:n]

	// 1. Check magic bytes (file signatures)
	if err := fv.checkMagicBytes(path, header); err != nil {
		return err
	}

	// 2. Check for binary data
	if fv.isBinaryData(header) {
		return errors.New("file appears to be binary (code extension on binary file)")
	}

	// 3. Language-specific validation
	if err := fv.validateCodeFile(path, header); err != nil {
		return err
	}

	return nil
}

// checkMagicBytes verifies file signature matches extension
func (fv *FileValidator) checkMagicBytes(path string, header []byte) error {
	ext := strings.ToLower(filepath.Ext(path))

	// File signatures (magic bytes)
	magicBytes := map[string][]byte{
		".png":  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
		".jpg":  {0xFF, 0xD8, 0xFF},
		".jpeg": {0xFF, 0xD8, 0xFF},
		".gif":  {0x47, 0x49, 0x46, 0x38, 0x39, 0x61},
		".pdf":  {0x25, 0x50, 0x44, 0x46, 0x2D},
		".zip":  {0x50, 0x4B, 0x03, 0x04},
		".exe":  {0x4D, 0x5A}, // PE executable
		".dll":  {0x4D, 0x5A}, // PE DLL
	}

	if magic, exists := magicBytes[ext]; exists {
		if !bytes.HasPrefix(header, magic) {
			return fmt.Errorf("magic bytes don't match %s extension (file may be disguised)", ext)
		}
	}

	return nil
}

// isBinaryData checks if file contains binary data
func (fv *FileValidator) isBinaryData(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range data {
		// Control characters (0-31 except tab, LF, CR)
		// and DEL (127)
		// Note: b is uint8, so b >= 0 is always true
		if b < 9 || (b > 13 && b < 32) || b == 127 {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider binary
	ratio := float64(nonPrintable) / float64(len(data))
	return ratio > 0.3
}

// validateCodeFile checks if file contains valid code for its extension
func (fv *FileValidator) validateCodeFile(path string, header []byte) error {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return fv.validateGoFile(header)
	case ".js", ".jsx":
		return fv.validateJSFile(header)
	case ".ts", ".tsx":
		return fv.validateTypeScriptFile(header)
	case ".py":
		return fv.validatePythonFile(header)
	case ".java":
		return fv.validateJavaFile(header)
	case ".c", ".h":
		return fv.validateCFile(header)
	case ".cpp", ".hpp", ".cc", ".cxx":
		return fv.validateCppFile(header)
	case ".rs":
		return fv.validateRustFile(header)
	case ".php":
		return fv.validatePHPFile(header)
	case ".rb":
		return fv.validateRubyFile(header)
	case ".swift":
		return fv.validateSwiftFile(header)
	}

	return nil
}

// validateGoFile checks for Go-specific patterns
func (fv *FileValidator) validateGoFile(header []byte) error {
	goPatterns := [][]byte{
		[]byte("package "),
		[]byte("import ("),
		[]byte("func "),
		[]byte("type "),
		[]byte("var "),
		[]byte("const "),
		[]byte("//go:build"),
		[]byte("// +build"),
	}

	for _, pattern := range goPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no Go patterns found (package, import, func, type, etc.)")
}

// validateJSFile checks for JavaScript patterns
func (fv *FileValidator) validateJSFile(header []byte) error {
	jsPatterns := [][]byte{
		[]byte("function "),
		[]byte("const "),
		[]byte("let "),
		[]byte("var "),
		[]byte("=>"),
		[]byte("import "),
		[]byte("export "),
		[]byte("class "),
		[]byte("if ("),
		[]byte("for ("),
		[]byte("document."),
		[]byte("console.log"),
	}

	for _, pattern := range jsPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no JavaScript patterns found")
}

// validateTypeScriptFile checks for TypeScript patterns
func (fv *FileValidator) validateTypeScriptFile(header []byte) error {
	// TS is similar to JS but may have type annotations
	tsPatterns := [][]byte{
		[]byte("interface "),
		[]byte("type "),
		[]byte("enum "),
		[]byte("namespace "),
		[]byte(": string"),
		[]byte(": number"),
		[]byte(": boolean"),
		[]byte("<T>"),
		[]byte("export "),
		[]byte("import "),
	}

	for _, pattern := range tsPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	// Fallback to JS validation
	return fv.validateJSFile(header)
}

// validatePythonFile checks for Python patterns
func (fv *FileValidator) validatePythonFile(header []byte) error {
	pythonPatterns := [][]byte{
		[]byte("def "),
		[]byte("import "),
		[]byte("from "),
		[]byte("class "),
		[]byte("if __name__"),
		[]byte("#!/usr/bin/python"),
		[]byte("self."),
		[]byte("None"),
		[]byte("True"),
		[]byte("False"),
	}

	for _, pattern := range pythonPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no Python patterns found")
}

// validateJavaFile checks for Java patterns
func (fv *FileValidator) validateJavaFile(header []byte) error {
	javaPatterns := [][]byte{
		[]byte("public class "),
		[]byte("private class "),
		[]byte("class "),
		[]byte("public static void main"),
		[]byte("import java."),
		[]byte("package "),
		[]byte("public "),
		[]byte("private "),
		[]byte("protected "),
	}

	for _, pattern := range javaPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no Java patterns found")
}

// validateCFile checks for C patterns
func (fv *FileValidator) validateCFile(header []byte) error {
	cPatterns := [][]byte{
		[]byte("#include "),
		[]byte("int main"),
		[]byte("void "),
		[]byte("char *"),
		[]byte("int "),
		[]byte("struct "),
		[]byte("typedef "),
		[]byte("enum "),
	}

	for _, pattern := range cPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no C patterns found")
}

// validateCppFile checks for C++ patterns
func (fv *FileValidator) validateCppFile(header []byte) error {
	cppPatterns := [][]byte{
		[]byte("#include <"),
		[]byte("class "),
		[]byte("template <"),
		[]byte("std::"),
		[]byte("namespace "),
		[]byte("cout <<"),
		[]byte("cin >>"),
		[]byte("virtual "),
		[]byte("public:"),
		[]byte("private:"),
		[]byte("protected:"),
	}

	for _, pattern := range cppPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no C++ patterns found")
}

// validateRustFile checks for Rust patterns
func (fv *FileValidator) validateRustFile(header []byte) error {
	rustPatterns := [][]byte{
		[]byte("fn main"),
		[]byte("let "),
		[]byte("fn "),
		[]byte("struct "),
		[]byte("enum "),
		[]byte("impl "),
		[]byte("use "),
		[]byte("pub "),
		[]byte("mut "),
	}

	for _, pattern := range rustPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no Rust patterns found")
}

// validatePHPFile checks for PHP patterns
func (fv *FileValidator) validatePHPFile(header []byte) error {
	phpPatterns := [][]byte{
		[]byte("<?php"),
		[]byte("<?="),
		[]byte("$"),
		[]byte("function "),
		[]byte("class "),
		[]byte("echo "),
		[]byte("print "),
		[]byte("foreach "),
		[]byte("array("),
	}

	for _, pattern := range phpPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no PHP patterns found")
}

// validateRubyFile checks for Ruby patterns
func (fv *FileValidator) validateRubyFile(header []byte) error {
	rubyPatterns := [][]byte{
		[]byte("def "),
		[]byte("class "),
		[]byte("module "),
		[]byte("puts "),
		[]byte("print "),
		[]byte("require "),
		[]byte("attr_accessor"),
		[]byte("end"),
	}

	for _, pattern := range rubyPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no Ruby patterns found")
}

// validateSwiftFile checks for Swift patterns
func (fv *FileValidator) validateSwiftFile(header []byte) error {
	swiftPatterns := [][]byte{
		[]byte("import "),
		[]byte("func "),
		[]byte("class "),
		[]byte("struct "),
		[]byte("enum "),
		[]byte("var "),
		[]byte("let "),
		[]byte("if let "),
		[]byte("guard let "),
	}

	for _, pattern := range swiftPatterns {
		if bytes.Contains(header, pattern) {
			return nil
		}
	}

	return errors.New("no Swift patterns found")
}
