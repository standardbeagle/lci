package mcp

import "fmt"

// Validation wrappers - common logic for converting validation results to errors

// validateWithValidator is a generic helper that validates using a RequestValidator and returns the first error
func validateWithValidator(validator *RequestValidator, request interface{}) error {
	result := validator.Validate(request)

	if !result.Valid {
		// Return the first validation error for backward compatibility
		err := result.Errors[0]
		return ValidationError{
			Field:   err.Field,
			Message: err.Message,
			Value:   err.Value,
			Code:    err.Code,
			Context: err.Context,
		}
	}

	return nil
}

// validateWithBusinessLogic is a generic helper that validates using a RequestValidator and applies business logic validation
func validateWithBusinessLogic(validator *RequestValidator, request interface{}, businessLogicValidation func(interface{}) *ValidationError) error {
	result := validator.Validate(request)

	if !result.Valid {
		// Return the first validation error for backward compatibility
		err := result.Errors[0]
		return ValidationError{
			Field:   err.Field,
			Message: err.Message,
			Value:   err.Value,
			Code:    err.Code,
			Context: err.Context,
		}
	}

	// Validate business logic
	if bizErr := businessLogicValidation(request); bizErr != nil {
		return ValidationError{
			Field:   bizErr.Field,
			Message: bizErr.Message,
			Value:   bizErr.Value,
			Code:    bizErr.Code,
			Context: bizErr.Context,
		}
	}

	return nil
}

// validateSearchParams validates search parameters using the enhanced framework
func validateSearchParams(params SearchParams) error {
	validator := CreateSearchValidator()

	return validateWithBusinessLogic(validator, params, func(request interface{}) *ValidationError {
		return ValidateSearchBusinessLogic(request.(SearchParams))
	})
}

// validateSymbolParams validates symbol parameters using the enhanced framework
func validateSymbolParams(params SymbolParams) error {
	validator := CreateSymbolValidator()
	return validateWithValidator(validator, params)
}

// validateTreeParams validates tree generation parameters using the enhanced framework
func validateTreeParams(params TreeParams) error {
	validator := CreateTreeValidator()
	return validateWithValidator(validator, params)
}

// validateIndexSwitchParams validates index switch parameters
func validateIndexSwitchParams(params IndexSwitchParams) error {
	if err := validateStringLength(params.Implementation, 1, 0, "implementation"); err != nil {
		return err
	}

	// Validate against known implementations
	validImplementations := []string{"modular-simple"}
	for _, validImpl := range validImplementations {
		if params.Implementation == validImpl {
			return nil
		}
	}

	return ValidationError{
		Field:   "implementation",
		Message: fmt.Sprintf("invalid implementation '%s'. Valid implementations: %s", params.Implementation, joinStrings(validImplementations)),
		Value:   params.Implementation,
		Code:    string(ErrCodeInvalidEnum),
		Context: map[string]interface{}{"valid_values": validImplementations},
	}
}

// validateIndexCompareParams validates index compare parameters
func validateIndexCompareParams(params IndexCompareParams) error {
	validator := NewRequestValidator()
	validator.AddRule("pattern", RequiredRule())
	validator.AddRule("pattern", MinLengthRule(1))
	validator.AddRule("pattern", MaxLengthRule(1000))
	validator.AddRule("max_lines", RangeRule(0, 50))

	return validateWithValidator(validator, params)
}

// validateObjectContextParams validates object context parameters using the enhanced framework
func validateObjectContextParams(params ObjectContextParams) error {
	validator := CreateObjectContextValidator()

	return validateWithBusinessLogic(validator, params, func(request interface{}) *ValidationError {
		return ValidateObjectContextBusinessLogic(request.(ObjectContextParams))
	})
}

// helper function to join strings with commas
func joinStrings(strings []string) string {
	result := ""
	for i, s := range strings {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
