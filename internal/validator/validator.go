package validator

import (
	"encoding/json"
	"fmt"
)

// Validator validates extracted data against expected schema
type Validator struct {
	// In the future, this will use CUE for full schema validation
	// For now, we do basic structural validation
}

// New creates a new Validator
func New() *Validator {
	return &Validator{}
}

// Validate checks that the input data has the expected structure
func (v *Validator) Validate(data interface{}) error {
	// Marshal to JSON and back to verify structure
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling data: %w", err)
	}

	// Verify it's valid JSON
	var check map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &check); err != nil {
		return fmt.Errorf("invalid JSON structure: %w", err)
	}

	// Check required fields exist
	requiredFields := []string{"entities", "architectures", "packages", "symbols"}
	for _, field := range requiredFields {
		if _, ok := check[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Validate entities have required fields
	if entities, ok := check["entities"].([]interface{}); ok {
		for i, e := range entities {
			entity, ok := e.(map[string]interface{})
			if !ok {
				return fmt.Errorf("entity[%d]: invalid structure", i)
			}
			if _, ok := entity["name"]; !ok {
				return fmt.Errorf("entity[%d]: missing 'name' field", i)
			}
			if _, ok := entity["file"]; !ok {
				return fmt.Errorf("entity[%d]: missing 'file' field", i)
			}
		}
	}

	return nil
}

// ValidateJSON validates JSON bytes directly
func (v *Validator) ValidateJSON(jsonBytes []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}
	return v.Validate(data)
}
