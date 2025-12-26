package mcp

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// symbolTypeValidator is a cached instance of SymbolTypeResolver for performance
// This avoids creating a new resolver instance on every validation call
var symbolTypeValidator = NewSymbolTypeResolver()

// ValidationError represents a parameter validation error
type ValidationError struct {
	Field   string                 `json:"field"`
	Message string                 `json:"message"`
	Value   interface{}            `json:"value,omitempty"`
	Code    string                 `json:"code"`
	Context map[string]interface{} `json:"context,omitempty"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// ValidationErrorCode represents standardized validation error codes
type ValidationErrorCode string

const (
	ErrCodeRequired      ValidationErrorCode = "REQUIRED"
	ErrCodeInvalid       ValidationErrorCode = "INVALID"
	ErrCodeTooLong       ValidationErrorCode = "TOO_LONG"
	ErrCodeTooShort      ValidationErrorCode = "TOO_SHORT"
	ErrCodeOutOfRange    ValidationErrorCode = "OUT_OF_RANGE"
	ErrCodeInvalidFormat ValidationErrorCode = "INVALID_FORMAT"
	ErrCodeInvalidEnum   ValidationErrorCode = "INVALID_ENUM"
	ErrCodeConflict      ValidationErrorCode = "CONFLICT"
)

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// RequestValidator provides comprehensive request validation
type RequestValidator struct {
	rules map[string][]ValidationRule
}

// ValidationRule defines a validation rule
type ValidationRule struct {
	Name     string
	Validate func(value interface{}) *ValidationError
}

// NewRequestValidator creates a new request validator
func NewRequestValidator() *RequestValidator {
	return &RequestValidator{
		rules: make(map[string][]ValidationRule),
	}
}

// AddRule adds a validation rule for a field
func (rv *RequestValidator) AddRule(field string, rule ValidationRule) {
	rv.rules[field] = append(rv.rules[field], rule)
}

// Validate validates a request struct
func (rv *RequestValidator) Validate(request interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true}

	v := reflect.ValueOf(request)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "request",
			Message: "request must be a struct",
			Code:    string(ErrCodeInvalid),
		})
		result.Valid = false
		return result
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Get JSON tag name if available (use strings.Cut for zero-copy)
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Extract first part before comma using strings.Cut
			if name, _, found := strings.Cut(jsonTag, ","); found {
				fieldName = name
			} else {
				fieldName = jsonTag
			}
		}

		// Apply validation rules for this field
		if rules, exists := rv.rules[fieldName]; exists {
			for _, rule := range rules {
				if err := rule.Validate(fieldValue.Interface()); err != nil {
					// Only set field if the error doesn't already have one set
					// This allows rules to set specific field names like "object_ids[0]"
					if err.Field == "" {
						err.Field = fieldName
					}
					result.Errors = append(result.Errors, *err)
					result.Valid = false
				}
			}
		}
	}

	return result
}

// Built-in validation rules

// RequiredRule creates a required field validation rule
func RequiredRule() ValidationRule {
	return ValidationRule{
		Name: "required",
		Validate: func(value interface{}) *ValidationError {
			if value == nil {
				return &ValidationError{
					Message: "field is required",
					Code:    string(ErrCodeRequired),
				}
			}

			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) == "" {
					return &ValidationError{
						Message: "field cannot be empty",
						Code:    string(ErrCodeRequired),
					}
				}
			case []string:
				if len(v) == 0 {
					return &ValidationError{
						Message: "field cannot be empty array",
						Code:    string(ErrCodeRequired),
					}
				}
			default:
				// For other types, check if it's the zero value
				if reflect.ValueOf(value).IsZero() {
					return &ValidationError{
						Message: "field is required",
						Code:    string(ErrCodeRequired),
					}
				}
			}

			return nil
		},
	}
}

// MinLengthRule creates a minimum length validation rule
func MinLengthRule(minLength int) ValidationRule {
	return ValidationRule{
		Name: "min_length",
		Validate: func(value interface{}) *ValidationError {
			if value == nil {
				return nil
			}

			// Check if it's a string
			if _, ok := value.(string); ok {
				return validateStringLength(value, minLength, 0, "")
			}

			// Check if it's a string array
			if _, ok := value.([]string); ok {
				return validateStringArrayLength(value, minLength, 0, "")
			}

			return &ValidationError{
				Message: "value must be a string or array of strings",
				Code:    string(ErrCodeInvalidFormat),
			}
		},
	}
}

// MaxLengthRule creates a maximum length validation rule
func MaxLengthRule(maxLength int) ValidationRule {
	return ValidationRule{
		Name: "max_length",
		Validate: func(value interface{}) *ValidationError {
			if value == nil {
				return nil
			}

			// Check if it's a string
			if _, ok := value.(string); ok {
				return validateStringLength(value, 0, maxLength, "")
			}

			// Check if it's a string array
			if _, ok := value.([]string); ok {
				return validateStringArrayLength(value, 0, maxLength, "")
			}

			return &ValidationError{
				Message: "value must be a string or array of strings",
				Code:    string(ErrCodeInvalidFormat),
			}
		},
	}
}

// RangeRule creates a numeric range validation rule
func RangeRule(min, max int) ValidationRule {
	return ValidationRule{
		Name: "range",
		Validate: func(value interface{}) *ValidationError {
			return validateInt32Range(value, min, max, "")
		},
	}
}

// EnumRule creates an enumeration validation rule
func EnumRule(validValues []string) ValidationRule {
	return ValidationRule{
		Name: "enum",
		Validate: func(value interface{}) *ValidationError {
			if value == nil {
				return nil // Skip nil values for optional fields
			}

			var stringValue string
			var ok bool

			switch v := value.(type) {
			case string:
				stringValue, ok = v, true
			default:
				return &ValidationError{
					Message: "value must be a string",
					Value:   value,
					Code:    string(ErrCodeInvalidFormat),
				}
			}

			if !ok {
				return &ValidationError{
					Message: "invalid string value",
					Value:   value,
					Code:    string(ErrCodeInvalid),
				}
			}

			for _, validValue := range validValues {
				if stringValue == validValue {
					return nil
				}
			}

			return &ValidationError{
				Message: "value must be one of: " + strings.Join(validValues, ", "),
				Value:   stringValue,
				Code:    string(ErrCodeInvalidEnum),
				Context: map[string]interface{}{"valid_values": validValues},
			}
		},
	}
}

// RegexRule creates a regular expression validation rule
func RegexRule(pattern string, message string) ValidationRule {
	regex := regexp.MustCompile(pattern)
	if message == "" {
		message = "value must match pattern: " + pattern
	}

	return ValidationRule{
		Name: "regex",
		Validate: func(value interface{}) *ValidationError {
			if value == nil {
				return nil // Skip nil values for optional fields
			}

			stringValue, ok := value.(string)
			if !ok {
				return &ValidationError{
					Message: "value must be a string",
					Value:   value,
					Code:    string(ErrCodeInvalidFormat),
				}
			}

			if !regex.MatchString(stringValue) {
				return &ValidationError{
					Message: message,
					Value:   stringValue,
					Code:    string(ErrCodeInvalidFormat),
					Context: map[string]interface{}{"pattern": pattern},
				}
			}

			return nil
		},
	}
}

// NoConflictRule creates a "no conflict" validation rule for mutually exclusive fields
func NoConflictRule(conflictFields []string) ValidationRule {
	return ValidationRule{
		Name: "no_conflict",
		Validate: func(value interface{}) *ValidationError {
			// This rule is handled at the struct level, not field level
			return nil
		},
	}
}

// CustomValidationRule creates a custom validation rule
func CustomValidationRule(name string, validator func(value interface{}) *ValidationError) ValidationRule {
	return ValidationRule{
		Name:     name,
		Validate: validator,
	}
}

// Validator factory functions for common MCP request types

// CreateSearchValidator creates a validator for search requests
func CreateSearchValidator() *RequestValidator {
	validator := NewRequestValidator()

	// Pattern validation with custom error message (optional - either pattern or patterns must be provided via business logic)
	validator.AddRule("pattern", CustomValidationRule("pattern_format", func(value interface{}) *ValidationError {
		if value == nil {
			return nil
		}
		return validateStringLength(value, 1, 100000, "pattern")
	}))

	// Max validation - allow very large values but check for negative
	validator.AddRule("max", CustomValidationRule("max_non_negative", func(value interface{}) *ValidationError {
		return validateNonNegative(value, "max")
	}))

	// MaxPerFile validation - check for negative values
	validator.AddRule("max_per_file", CustomValidationRule("max_per_file_non_negative", func(value interface{}) *ValidationError {
		return validateNonNegative(value, "max_per_file")
	}))

	// Symbol types validation - uses SymbolTypeResolver for all 23 types plus aliases/fuzzy
	// Validation now passes through the resolver which handles exact, alias, prefix, and fuzzy matching
	validator.AddRule("symbol_types", CustomValidationRule("symbol_types_resolver", func(value interface{}) *ValidationError {
		// Use cached validator for performance
		resolver := symbolTypeValidator

		// Accept both array and string formats
		if str, ok := value.(string); ok {
			// Empty string means no filter - skip validation
			if strings.TrimSpace(str) == "" {
				return nil
			}

			// Use resolver to validate each type (zero-copy iteration)
			remaining := str
			for len(remaining) > 0 {
				var item string
				if idx := strings.IndexByte(remaining, ','); idx >= 0 {
					item = remaining[:idx]
					remaining = remaining[idx+1:]
				} else {
					item = remaining
					remaining = ""
				}
				trimmed := strings.TrimSpace(item)
				if trimmed == "" {
					continue
				}
				resolution := resolver.Resolve(trimmed)
				// Only reject if nothing resolved (not even fuzzy match)
				if resolution.Resolved == "" {
					return &ValidationError{
						Field:   "symbol_types",
						Message: fmt.Sprintf("invalid symbol type '%s'. Valid types: %s. Aliases: func, var, const, cls, meth, iface, def (Python), fn (Rust)", trimmed, strings.Join(CanonicalSymbolTypes, ", ")),
						Value:   trimmed,
						Code:    string(ErrCodeInvalidEnum),
					}
				}
			}
			return nil
		}

		// Handle array format
		if arr, ok := value.([]interface{}); ok {
			for _, item := range arr {
				if str, ok := item.(string); ok {
					resolution := resolver.Resolve(str)
					if resolution.Resolved == "" {
						return &ValidationError{
							Field:   "symbol_types",
							Message: fmt.Sprintf("invalid symbol type '%s'. Valid types: %s", str, strings.Join(CanonicalSymbolTypes, ", ")),
							Value:   str,
							Code:    string(ErrCodeInvalidEnum),
						}
					}
				}
			}
		}
		return nil
	}))

	return validator
}

// CreateSymbolValidator creates a validator for symbol requests
func CreateSymbolValidator() *RequestValidator {
	validator := NewRequestValidator()

	// Symbol validation with custom error message
	validator.AddRule("symbol", CustomValidationRule("symbol_required", func(value interface{}) *ValidationError {
		return validateRequiredString(value, "symbol")
	}))
	validator.AddRule("symbol", MinLengthRule(1))
	validator.AddRule("symbol", MaxLengthRule(100000)) // Allow very long symbols per test expectations

	return validator
}

// CreateTreeValidator creates a validator for tree requests
func CreateTreeValidator() *RequestValidator {
	validator := NewRequestValidator()

	// Function validation
	validator.AddRule("function", RequiredRule())
	validator.AddRule("function", MinLengthRule(1))
	validator.AddRule("function", MaxLengthRule(500))

	// Max depth validation
	validator.AddRule("max_depth", RangeRule(0, 50))

	return validator
}

// CreateObjectContextValidator creates a validator for object context requests
func CreateObjectContextValidator() *RequestValidator {
	validator := NewRequestValidator()

	return validator
}

// Struct-level validation functions for complex business logic

// ValidateSearchBusinessLogic validates business logic for search requests
func ValidateSearchBusinessLogic(params SearchParams) *ValidationError {
	// Validate that either pattern or patterns is provided
	hasPattern := strings.TrimSpace(params.Pattern) != ""
	hasPatterns := strings.TrimSpace(params.Patterns) != ""

	if !hasPattern && !hasPatterns {
		return &ValidationError{
			Field:   "pattern",
			Message: "pattern cannot be empty",
			Code:    string(ErrCodeRequired),
			Context: map[string]interface{}{
				"has_pattern":  hasPattern,
				"has_patterns": hasPatterns,
			},
		}
	}

	// Note: DeclarationOnly and UsageOnly are not supported in the new consolidated format
	// If needed, these can be added back as flags in the Flags field

	return nil
}

// ValidateObjectContextBusinessLogic validates business logic for object context requests
func ValidateObjectContextBusinessLogic(params ObjectContextParams) *ValidationError {
	hasID := strings.TrimSpace(params.ID) != ""
	hasName := strings.TrimSpace(params.Name) != ""

	// Must have either 'id' or 'name'
	if !hasID && !hasName {
		return &ValidationError{
			Field:   "id",
			Message: "missing required 'id' parameter with object ID from search results",
			Code:    string(ErrCodeRequired),
			Context: map[string]interface{}{
				"example":  `{"id": "VE"} or {"id": "VE,tG,Ab"} for multiple`,
				"workflow": "1. Search: {\"pattern\": \"X\"} → 2. Find o=XX → 3. Context: {\"id\": \"XX\"}",
				"common_mistakes": []string{
					"Using 'symbol_id' instead of 'id'",
					"Using 'object_ids' array - use 'id' with comma-separated values",
					"Using line numbers - use object ID from search",
				},
			},
		}
	}

	// Can't have both
	if hasID && hasName {
		return &ValidationError{
			Field:   "id,name",
			Message: "parameter conflict: use either 'id' OR 'name', not both",
			Code:    string(ErrCodeConflict),
			Context: map[string]interface{}{
				"recommendation": "Prefer 'id' parameter with object IDs from search results",
			},
		}
	}

	return nil
}

// Enhanced validation functions that use the new framework but maintain backward compatibility
// See validation_validators.go for implementation

// isValidObjectID checks if an object ID follows expected patterns
func isValidObjectID(objectID string) bool {
	// Base-63 (A-Za-z0-9_) - NO LEGACY SUPPORT
	if len(objectID) == 0 {
		return false
	}
	for _, c := range objectID {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// Structured error response functions for MCP handlers

// CreateValidationErrorResponse creates a structured validation error response
func CreateValidationErrorResponse(toolName string, validationError error, context map[string]interface{}) (*mcpsdk.CallToolResult, error) {
	if validationErr, ok := validationError.(ValidationError); ok {
		// Enhanced structured error with validation details
		errorResponse := map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"type":    "validation_error",
				"tool":    toolName,
				"code":    validationErr.Code,
				"field":   validationErr.Field,
				"message": validationErr.Message,
				"value":   validationErr.Value,
				"context": validationErr.Context,
			},
			"validation_details": map[string]interface{}{
				"error_code":     validationErr.Code,
				"field_name":     validationErr.Field,
				"error_message":  validationErr.Message,
				"provided_value": validationErr.Value,
				"help_context":   validationErr.Context,
			},
		}

		// Merge additional context
		if context != nil {
			if errorContext, exists := errorResponse["error"].(map[string]interface{}); exists {
				for k, v := range context {
					errorContext[k] = v
				}
			}
		}

		response, err := createJSONResponse(errorResponse)
		if err != nil {
			return nil, err
		}

		// CRITICAL: Set IsError=true per MCP SDK specification
		response.IsError = true

		return response, nil
	}

	// Fallback for non-ValidationError types
	return createSmartErrorResponse(toolName, validationError, context)
}

// CreateMultiValidationErrorResponse creates a response for multiple validation errors
func CreateMultiValidationErrorResponse(toolName string, errors []ValidationError, context map[string]interface{}) (*mcpsdk.CallToolResult, error) {
	errorDetails := make([]map[string]interface{}, len(errors))
	for i, err := range errors {
		errorDetails[i] = map[string]interface{}{
			"error_code":     err.Code,
			"field_name":     err.Field,
			"error_message":  err.Message,
			"provided_value": err.Value,
			"help_context":   err.Context,
		}
	}

	errorResponse := map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"type":    "multiple_validation_errors",
			"tool":    toolName,
			"message": fmt.Sprintf("Request failed with %d validation errors", len(errors)),
			"count":   len(errors),
		},
		"validation_errors": errorDetails,
	}

	// Merge additional context
	if context != nil {
		errorResponse["context"] = context
	}

	response, err := createJSONResponse(errorResponse)
	if err != nil {
		return nil, err
	}

	// CRITICAL: Set IsError=true per MCP SDK specification
	response.IsError = true

	return response, nil
}

// ValidateRequestWithResponse is a convenience function that validates and creates an error response if needed
func ValidateRequestWithResponse(toolName string, validator *RequestValidator, request interface{}, context map[string]interface{}) (*mcpsdk.CallToolResult, error) {
	result := validator.Validate(request)

	if !result.Valid {
		if len(result.Errors) == 1 {
			// Single error - use standard validation error response
			return CreateValidationErrorResponse(toolName, result.Errors[0], context)
		}
		// Multiple errors - use multi-error response
		return CreateMultiValidationErrorResponse(toolName, result.Errors, context)
	}

	// Validation passed - no response needed (caller should continue processing)
	return nil, nil
}

// Validation helper functions

// ValidateAndFormatError validates a request and formats any validation errors
func ValidateAndFormatError(validator *RequestValidator, request interface{}) error {
	result := validator.Validate(request)
	if !result.Valid {
		// Return the first error for backward compatibility
		return result.Errors[0]
	}
	return nil
}

// GetValidationSummary returns a human-readable summary of validation results
func GetValidationSummary(result *ValidationResult) string {
	if result.Valid {
		return "Validation passed"
	}

	if len(result.Errors) == 1 {
		err := result.Errors[0]
		return fmt.Sprintf("Validation failed: %s (field: %s, code: %s)", err.Message, err.Field, err.Code)
	}

	return fmt.Sprintf("Validation failed with %d errors. First error: %s (field: %s)",
		len(result.Errors), result.Errors[0].Message, result.Errors[0].Field)
}
