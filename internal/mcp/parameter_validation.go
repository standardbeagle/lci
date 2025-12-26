package mcp

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ValidateAndWarnExtraParams checks for unknown parameters in a JSON request
// and returns a list of warning messages for any extra parameters found.
// This allows graceful handling of unknown parameters while still processing the request.
func ValidateAndWarnExtraParams(jsonBytes []byte, validStruct interface{}) []string {
	var warnings []string

	// Parse JSON into a map to get all provided fields
	var providedFields map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &providedFields); err != nil {
		// If we can't parse as a map, we can't detect extra params
		// This is not an error - just return no warnings
		return warnings
	}

	// Get valid field names from the struct using reflection
	validFields := getValidFieldNames(validStruct)

	// Check for extra fields
	for fieldName := range providedFields {
		if !validFields[fieldName] {
			warnings = append(warnings, fmt.Sprintf("Unknown parameter '%s' was provided and will be ignored", fieldName))
		}
	}

	return warnings
}

// getValidFieldNames uses reflection to extract all valid field names from a struct
// It looks at json tags to get the actual JSON field names
func getValidFieldNames(structPtr interface{}) map[string]bool {
	validFields := make(map[string]bool)

	// Get the type of the struct
	structType := reflect.TypeOf(structPtr)
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	// If it's not a struct, return empty map
	if structType.Kind() != reflect.Struct {
		return validFields
	}

	// Iterate through all fields
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// Get the json tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			// If no json tag, use the field name in lowercase
			validFields[strings.ToLower(field.Name)] = true
		} else {
			// Parse the json tag (handle "field,omitempty" format)
			tagParts := strings.Split(jsonTag, ",")
			if tagParts[0] != "" && tagParts[0] != "-" {
				validFields[tagParts[0]] = true
			}
		}

		// Also add the original field name for flexibility
		validFields[field.Name] = true
	}

	return validFields
}

// ValidateRequiredParams checks that all required parameters are present
// This is complementary to ValidateAndWarnExtraParams
func ValidateRequiredParams(params interface{}) error {
	// Use reflection to check for required fields
	v := reflect.ValueOf(params)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// Check if field has a "required" tag
		required := field.Tag.Get("required")
		if required == "true" {
			// Check if the field is empty
			if isEmptyValue(value) {
				jsonTag := field.Tag.Get("json")
				fieldName := field.Name
				if jsonTag != "" {
					tagParts := strings.Split(jsonTag, ",")
					if tagParts[0] != "" {
						fieldName = tagParts[0]
					}
				}
				return fmt.Errorf("required parameter '%s' is missing", fieldName)
			}
		}
	}

	return nil
}

// isEmptyValue checks if a reflect.Value represents an empty/zero value
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}
