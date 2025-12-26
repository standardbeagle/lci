package mcp

import (
	"testing"
)

// TestValidationFrameworkIntegration tests the validation framework with realistic MCP scenarios
func TestValidationFrameworkIntegration(t *testing.T) {
	t.Run("structured error responses", func(t *testing.T) {
		// Test CreateValidationErrorResponse
		validationErr := ValidationError{
			Field:   "pattern",
			Message: "pattern cannot be empty",
			Value:   "",
			Code:    "REQUIRED",
			Context: map[string]interface{}{"min_length": 1},
		}

		response, err := CreateValidationErrorResponse("search", validationErr, map[string]interface{}{
			"request_id": "test-123",
		})

		if err != nil {
			t.Fatalf("CreateValidationErrorResponse failed: %v", err)
		}

		if response == nil {
			t.Fatal("Expected response but got nil")
		}

		// Verify response structure (basic check)
		t.Logf("Validation error response created successfully")
	})

	t.Run("multiple validation errors", func(t *testing.T) {
		errors := []ValidationError{
			{
				Field:   "pattern",
				Message: "pattern cannot be empty",
				Code:    "REQUIRED",
			},
			{
				Field:   "max_results",
				Message: "max_results cannot be negative",
				Value:   -1,
				Code:    "OUT_OF_RANGE",
			},
		}

		response, err := CreateMultiValidationErrorResponse("search", errors, nil)
		if err != nil {
			t.Fatalf("CreateMultiValidationErrorResponse failed: %v", err)
		}

		if response == nil {
			t.Fatal("Expected response but got nil")
		}

		t.Logf("Multi-validation error response created successfully")
	})

	t.Run("convenience validation function", func(t *testing.T) {
		validator := NewRequestValidator()
		validator.AddRule("test_field", RequiredRule())

		type TestRequest struct {
			TestField string `json:"test_field"`
		}

		// Test invalid request
		request := TestRequest{TestField: ""}

		response, err := ValidateRequestWithResponse("test_tool", validator, request, map[string]interface{}{
			"operation": "test",
		})

		if err != nil {
			t.Fatalf("ValidateRequestWithResponse failed: %v", err)
		}

		if response == nil {
			t.Fatal("Expected validation error response but got nil")
		}

		// Test valid request
		validRequest := TestRequest{TestField: "valid_value"}

		response, err = ValidateRequestWithResponse("test_tool", validator, validRequest, nil)
		if err != nil {
			t.Fatalf("ValidateRequestWithResponse failed for valid request: %v", err)
		}

		if response != nil {
			t.Fatal("Expected no response for valid request but got one")
		}

		t.Logf("Convenience validation function works correctly")
	})
}

// TestValidatorFactories tests the validator factory functions
func TestValidatorFactories(t *testing.T) {
	t.Run("CreateSearchValidator", func(t *testing.T) {
		validator := CreateSearchValidator()

		// Test valid search params
		validParams := struct {
			Pattern      string `json:"pattern"`
			Max          int    `json:"max_results"`
			MaxLineCount int    `json:"max_line_count"`
		}{
			Pattern:      "test pattern",
			Max:          100,
			MaxLineCount: 10,
		}

		result := validator.Validate(validParams)
		if !result.Valid {
			t.Errorf("Expected valid search params to pass validation, got errors: %v", result.Errors)
		}

		// Test invalid search params
		invalidParams := struct {
			Pattern      string `json:"pattern"`
			Max          int    `json:"max_results"`
			MaxLineCount int    `json:"max_line_count"`
		}{
			Pattern:      "",
			Max:          -1,
			MaxLineCount: 10,
		}

		result = validator.Validate(invalidParams)
		if result.Valid {
			t.Error("Expected invalid search params to fail validation")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected validation errors but got none")
		}

		t.Logf("CreateSearchValidator works correctly")
	})

	t.Run("CreateSymbolValidator", func(t *testing.T) {
		validator := CreateSymbolValidator()

		// Test valid symbol params
		validParams := struct {
			Symbol string `json:"symbol"`
		}{
			Symbol: "testFunction",
		}

		result := validator.Validate(validParams)
		if !result.Valid {
			t.Errorf("Expected valid symbol params to pass validation, got errors: %v", result.Errors)
		}

		// Test invalid symbol params
		invalidParams := struct {
			Symbol string `json:"symbol"`
		}{
			Symbol: "",
		}

		result = validator.Validate(invalidParams)
		if result.Valid {
			t.Error("Expected invalid symbol params to fail validation")
		}

		t.Logf("CreateSymbolValidator works correctly")
	})
}

// TestValidationRuleTypes tests the different validation rule types
func TestValidationRuleTypes(t *testing.T) {
	t.Run("built-in rules work correctly", func(t *testing.T) {
		// Test RequiredRule
		requiredRule := RequiredRule()
		if err := requiredRule.Validate("test"); err != nil {
			t.Errorf("RequiredRule failed for valid value: %s", err.Message)
		}
		if err := requiredRule.Validate(""); err == nil {
			t.Error("RequiredRule passed for empty string")
		}

		// Test MinLengthRule
		minLengthRule := MinLengthRule(3)
		if err := minLengthRule.Validate("test"); err != nil {
			t.Errorf("MinLengthRule failed for valid value: %s", err.Message)
		}
		if err := minLengthRule.Validate("ab"); err == nil {
			t.Error("MinLengthRule passed for too short string")
		}

		// Test MaxLengthRule
		maxLengthRule := MaxLengthRule(5)
		if err := maxLengthRule.Validate("test"); err != nil {
			t.Errorf("MaxLengthRule failed for valid value: %s", err.Message)
		}
		if err := maxLengthRule.Validate("toolong"); err == nil {
			t.Error("MaxLengthRule passed for too long string")
		}

		// Test RangeRule
		rangeRule := RangeRule(1, 10)
		if err := rangeRule.Validate(5); err != nil {
			t.Errorf("RangeRule failed for valid value: %s", err.Message)
		}
		if err := rangeRule.Validate(0); err == nil {
			t.Error("RangeRule passed for value below range")
		}
		if err := rangeRule.Validate(11); err == nil {
			t.Error("RangeRule passed for value above range")
		}

		// Test EnumRule
		enumRule := EnumRule([]string{"option1", "option2", "option3"})
		if err := enumRule.Validate("option1"); err != nil {
			t.Errorf("EnumRule failed for valid value: %s", err.Message)
		}
		if err := enumRule.Validate("invalid"); err == nil {
			t.Error("EnumRule passed for invalid value")
		}

		t.Logf("Built-in validation rules work correctly")
	})

	t.Run("custom rules work correctly", func(t *testing.T) {
		// Test custom validation rule
		customRule := CustomValidationRule("custom_test", func(value interface{}) *ValidationError {
			str, ok := value.(string)
			if !ok {
				return &ValidationError{
					Message: "value must be a string",
					Code:    "INVALID_FORMAT",
				}
			}
			if str != "expected" {
				return &ValidationError{
					Message: "value must be 'expected'",
					Value:   str,
					Code:    "INVALID_VALUE",
				}
			}
			return nil
		})

		if err := customRule.Validate("expected"); err != nil {
			t.Errorf("Custom rule failed for valid value: %s", err.Message)
		}
		if err := customRule.Validate("wrong"); err == nil {
			t.Error("Custom rule passed for invalid value")
		}
		if err := customRule.Validate(123); err == nil {
			t.Error("Custom rule passed for wrong type")
		}

		t.Logf("Custom validation rules work correctly")
	})
}

// TestValidationHelperFunctions tests the helper functions
func TestValidationHelperFunctions(t *testing.T) {
	t.Run("GetValidationSummary", func(t *testing.T) {
		// Test valid result
		validResult := &ValidationResult{
			Valid:  true,
			Errors: []ValidationError{},
		}
		summary := GetValidationSummary(validResult)
		if summary != "Validation passed" {
			t.Errorf("Expected 'Validation passed', got '%s'", summary)
		}

		// Test single error
		singleErrorResult := &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "test",
					Message: "test error",
					Code:    "TEST_ERROR",
				},
			},
		}
		summary = GetValidationSummary(singleErrorResult)
		expected := "Validation failed: test error (field: test, code: TEST_ERROR)"
		if summary != expected {
			t.Errorf("Expected '%s', got '%s'", expected, summary)
		}

		// Test multiple errors
		multipleErrorResult := &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Field:   "test1",
					Message: "first error",
					Code:    "FIRST_ERROR",
				},
				{
					Field:   "test2",
					Message: "second error",
					Code:    "SECOND_ERROR",
				},
			},
		}
		summary = GetValidationSummary(multipleErrorResult)
		expected = "Validation failed with 2 errors. First error: first error (field: test1)"
		if summary != expected {
			t.Errorf("Expected '%s', got '%s'", expected, summary)
		}

		t.Logf("GetValidationSummary works correctly")
	})

	t.Run("ValidateAndFormatError", func(t *testing.T) {
		validator := NewRequestValidator()
		validator.AddRule("required_field", RequiredRule())

		type TestStruct struct {
			RequiredField string `json:"required_field"`
		}

		// Test valid struct
		validStruct := TestStruct{RequiredField: "test"}
		err := ValidateAndFormatError(validator, validStruct)
		if err != nil {
			t.Errorf("ValidateAndFormatError failed for valid struct: %v", err)
		}

		// Test invalid struct
		invalidStruct := TestStruct{RequiredField: ""}
		err = ValidateAndFormatError(validator, invalidStruct)
		if err == nil {
			t.Error("ValidateAndFormatError passed for invalid struct")
		}

		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Error("Expected ValidationError but got different type")
		}
		if validationErr.Field != "required_field" {
			t.Errorf("Expected field 'required_field', got '%s'", validationErr.Field)
		}

		t.Logf("ValidateAndFormatError works correctly")
	})
}
