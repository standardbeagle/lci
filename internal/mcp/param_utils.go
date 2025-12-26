package mcp

import "encoding/json"

// collectUnknownFields parses raw JSON into a map, capturing any fields
// that aren't part of the provided known field set. Optionally, nested field
// validation can be supplied to inspect nested objects (e.g., weights.thresholds).
func collectUnknownFields(
	data []byte,
	known map[string]struct{},
	nested map[string]map[string]struct{},
) (map[string]json.RawMessage, []UnknownField, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, err
	}

	var warnings []UnknownField
	for key, value := range raw {
		if _, ok := known[key]; !ok {
			warnings = append(warnings, decodeUnknownField(key, value))
			continue
		}

		if nested != nil {
			if nestedFields, ok := nested[key]; ok {
				warnings = append(warnings, collectNestedUnknownFields(key, value, nestedFields)...)
			}
		}
	}

	return raw, warnings, nil
}

func decodeUnknownField(name string, data json.RawMessage) UnknownField {
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		value = string(data)
	}
	return UnknownField{Name: name, Value: value}
}

func collectNestedUnknownFields(parent string, data json.RawMessage, allowed map[string]struct{}) []UnknownField {
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(data, &nested); err != nil {
		return nil
	}

	var warnings []UnknownField
	for key, value := range nested {
		if _, ok := allowed[key]; ok {
			continue
		}

		var nestedValue interface{}
		if err := json.Unmarshal(value, &nestedValue); err != nil {
			nestedValue = string(value)
		}

		warnings = append(warnings, UnknownField{
			Name:  parent + "." + key,
			Value: nestedValue,
		})
	}

	return warnings
}
