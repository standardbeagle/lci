package mcp

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Validation helpers - common validation logic extracted to reduce duplication

// validateNonNegative validates that a numeric value is non-negative
func validateNonNegative(value interface{}, fieldName string) *ValidationError {
	var intValue int64
	switch v := value.(type) {
	case int:
		if v < 0 {
			return &ValidationError{
				Field:   fieldName,
				Message: fieldName + " cannot be negative",
				Value:   v,
				Code:    string(ErrCodeOutOfRange),
			}
		}
		intValue = int64(v)
		// Check for overflow on int values too
		if intValue > 2147483647 {
			return &ValidationError{
				Field:   fieldName,
				Message: "value exceeds 32-bit integer range",
				Value:   intValue,
				Code:    string(ErrCodeOutOfRange),
			}
		}
	case float64:
		if v < 0 {
			return &ValidationError{
				Field:   fieldName,
				Message: fieldName + " cannot be negative",
				Value:   v,
				Code:    string(ErrCodeOutOfRange),
			}
		}
		intValue = int64(v)
		// Check for overflow on float64 values
		if v > 2147483647 {
			return &ValidationError{
				Field:   fieldName,
				Message: "value exceeds 32-bit integer range",
				Value:   v,
				Code:    string(ErrCodeOutOfRange),
			}
		}
	default:
		return &ValidationError{
			Field:   fieldName,
			Message: fieldName + " must be a number",
			Value:   value,
			Code:    string(ErrCodeInvalidFormat),
		}
	}

	if intValue < 0 {
		return &ValidationError{
			Field:   fieldName,
			Message: fieldName + " cannot be negative",
			Value:   intValue,
			Code:    string(ErrCodeOutOfRange),
		}
	}

	return nil
}

// validateInt32Range validates that an integer is within a specified range
func validateInt32Range(value interface{}, min, max int, fieldName string) *ValidationError {
	if value == nil {
		return nil
	}

	var intValue int64
	var ok bool

	switch v := value.(type) {
	case int:
		intValue, ok = int64(v), true
	case float64:
		// Check if the float64 value is an integer and within range
		if v > float64(max) || v < float64(min) {
			return &ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("value must be between %d and %d", min, max),
				Value:   v,
				Code:    string(ErrCodeOutOfRange),
				Context: map[string]interface{}{"min": min, "max": max},
			}
		}
		// Check for overflow
		if v > float64(2147483647) || v < float64(-2147483648) {
			return &ValidationError{
				Field:   fieldName,
				Message: "value exceeds 32-bit integer range",
				Value:   v,
				Code:    string(ErrCodeOutOfRange),
			}
		}
		intValue, ok = int64(v), true
	default:
		return &ValidationError{
			Field:   fieldName,
			Message: "value must be a number",
			Value:   value,
			Code:    string(ErrCodeInvalidFormat),
		}
	}

	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Message: "invalid numeric value",
			Value:   value,
			Code:    string(ErrCodeInvalid),
		}
	}

	// Check if value exceeds int32 range
	if intValue > 2147483647 || intValue < -2147483648 {
		return &ValidationError{
			Field:   fieldName,
			Message: "value exceeds 32-bit integer range",
			Value:   intValue,
			Code:    string(ErrCodeOutOfRange),
		}
	}

	if intValue < int64(min) || intValue > int64(max) {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("value must be between %d and %d", min, max),
			Value:   intValue,
			Code:    string(ErrCodeOutOfRange),
			Context: map[string]interface{}{"min": min, "max": max},
		}
	}

	return nil
}

// validateStringLength validates string length with configurable min/max
func validateStringLength(value interface{}, minLength, maxLength int, fieldName string) *ValidationError {
	if value == nil {
		return nil
	}

	strValue, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Message: fieldName + " must be a string",
			Value:   value,
			Code:    string(ErrCodeInvalidFormat),
		}
	}

	length := utf8.RuneCountInString(strValue)

	if minLength > 0 && length < minLength {
		var msg string
		if fieldName == "pattern" {
			msg = "pattern cannot be empty"
		} else {
			msg = fmt.Sprintf("%s must have at least %d characters", fieldName, minLength)
		}
		return &ValidationError{
			Field:   fieldName,
			Message: msg,
			Value:   strValue,
			Code:    string(ErrCodeTooShort),
			Context: map[string]interface{}{"min_length": minLength, "actual_length": length},
		}
	}

	if maxLength > 0 && length > maxLength {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s is too long (max %d characters)", fieldName, maxLength),
			Value:   strValue,
			Code:    string(ErrCodeTooLong),
			Context: map[string]interface{}{"max_length": maxLength, "actual_length": length},
		}
	}

	return nil
}

// validateStringArrayLength validates array length with configurable min/max
func validateStringArrayLength(value interface{}, minLength, maxLength int, fieldName string) *ValidationError {
	if value == nil {
		return nil
	}

	arrayValue, ok := value.([]string)
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Message: "value must be an array of strings",
			Value:   value,
			Code:    string(ErrCodeInvalidFormat),
		}
	}

	length := len(arrayValue)

	if minLength > 0 && length < minLength {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("minimum array length is %d", minLength),
			Value:   arrayValue,
			Code:    string(ErrCodeTooShort),
			Context: map[string]interface{}{"min_length": minLength, "actual_length": length},
		}
	}

	if maxLength > 0 && length > maxLength {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("maximum array length is %d", maxLength),
			Value:   arrayValue,
			Code:    string(ErrCodeTooLong),
			Context: map[string]interface{}{"max_length": maxLength, "actual_length": length},
		}
	}

	return nil
}

// validateStringArray validates that all elements in a string array match valid values
func validateStringArray(value interface{}, validValues []string, fieldName string) *ValidationError {
	if value == nil {
		return nil
	}

	arrayValue, ok := value.([]string)
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Message: "value must be an array of strings",
			Value:   value,
			Code:    string(ErrCodeInvalidFormat),
		}
	}

	for _, item := range arrayValue {
		if !stringInSlice(validValues, item) {
			var msg string
			if fieldName == "symbol_types" {
				msg = fmt.Sprintf("invalid symbol type '%s'", item)
			} else {
				msg = fmt.Sprintf("invalid value '%s'. Valid values: %s", item, strings.Join(validValues, ", "))
			}
			return &ValidationError{
				Field:   fieldName,
				Message: msg,
				Value:   item,
				Code:    string(ErrCodeInvalidEnum),
				Context: map[string]interface{}{"valid_values": validValues},
			}
		}
	}

	return nil
}

// validateRequiredString validates that a string field is not empty
func validateRequiredString(value interface{}, fieldName string) *ValidationError {
	if value == nil {
		return &ValidationError{
			Field:   fieldName,
			Message: fieldName + " cannot be empty",
			Code:    string(ErrCodeRequired),
		}
	}

	strValue, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Message: fieldName + " must be a string",
			Value:   value,
			Code:    string(ErrCodeInvalidFormat),
		}
	}

	if strings.TrimSpace(strValue) == "" {
		return &ValidationError{
			Field:   fieldName,
			Message: fieldName + " cannot be empty",
			Code:    string(ErrCodeRequired),
		}
	}

	return nil
}

// validateStringArrayWithNonEmpty validates a string array where each element must be non-empty
func validateStringArrayWithNonEmpty(value interface{}, fieldName string) *ValidationError {
	if value == nil {
		return nil // Optional field
	}

	arrayValue, ok := value.([]string)
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Message: "value must be an array of strings",
			Value:   value,
			Code:    string(ErrCodeInvalidFormat),
		}
	}

	for i, item := range arrayValue {
		if strings.TrimSpace(item) == "" {
			return &ValidationError{
				Field:   fmt.Sprintf("%s[%d]", fieldName, i),
				Message: "value cannot be empty",
				Code:    string(ErrCodeRequired),
			}
		}
	}

	return nil
}

// validateStringArrayWithPattern validates string array elements match a regex pattern
func validateStringArrayWithPattern(value interface{}, pattern string, message string, fieldName string) *ValidationError {
	if value == nil {
		return nil // Optional field
	}

	regex := regexp.MustCompile(pattern)
	if message == "" {
		message = "value must match pattern: " + pattern
	}

	arrayValue, ok := value.([]string)
	if !ok {
		return &ValidationError{
			Field:   fieldName,
			Message: "value must be an array of strings",
			Value:   value,
			Code:    string(ErrCodeInvalidFormat),
		}
	}

	for i, item := range arrayValue {
		if !regex.MatchString(item) {
			return &ValidationError{
				Field:   fmt.Sprintf("%s[%d]", fieldName, i),
				Message: message,
				Value:   item,
				Code:    string(ErrCodeInvalidFormat),
				Context: map[string]interface{}{"pattern": pattern},
			}
		}
	}

	return nil
}

// helper function to check if a slice contains a value
func stringInSlice(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
