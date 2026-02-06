package validator

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCUEContractEnforcement demonstrates the CUE contract validation.
// This ensures "silent failures" cannot happen in the policy engine.
func TestCUEContractEnforcement(t *testing.T) {
	v, err := New()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		data    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid_input",
			data: map[string]interface{}{
				"entities":      []interface{}{},
				"architectures": []interface{}{},
				"packages":      []interface{}{},
				"components":    []interface{}{},
				"signals":       []interface{}{},
				"ports":         []interface{}{},
				"dependencies":  []interface{}{},
				"symbols":       []interface{}{},
			},
			wantErr: false,
		},
		{
			name: "missing_entities_field",
			data: map[string]interface{}{
				// CUE allows missing fields by default (open struct behavior)
				// The important validation is that PRESENT fields match the schema
				"architectures": []interface{}{},
				"packages":      []interface{}{},
				"components":    []interface{}{},
				"signals":       []interface{}{},
				"ports":         []interface{}{},
				"dependencies":  []interface{}{},
				"symbols":       []interface{}{},
			},
			wantErr: false, // CUE allows missing fields in open structs
		},
		{
			name: "invalid_port_direction",
			data: map[string]interface{}{
				"entities":      []interface{}{},
				"architectures": []interface{}{},
				"packages":      []interface{}{},
				"components":    []interface{}{},
				"signals":       []interface{}{},
				"ports": []interface{}{
					map[string]interface{}{
						"name":      "bad_port",
						"direction": "invalid_direction", // Not in enum!
						"type":      "std_logic",
						"file":      "test.vhd",
						"line":      1,
						"in_entity": "test",
					},
				},
				"dependencies": []interface{}{},
				"symbols":      []interface{}{},
			},
			wantErr: true, // CUE catches this!
		},
		{
			name: "empty_signal_type",
			data: map[string]interface{}{
				"entities":      []interface{}{},
				"architectures": []interface{}{},
				"packages":      []interface{}{},
				"components":    []interface{}{},
				"signals": []interface{}{
					map[string]interface{}{
						"name":      "bad_signal",
						"type":      "", // Empty type - schema says type != ""
						"file":      "test.vhd",
						"line":      1,
						"in_entity": "test",
					},
				},
				"ports":        []interface{}{},
				"dependencies": []interface{}{},
				"symbols":      []interface{}{},
			},
			wantErr: true, // CUE catches this!
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNoSecondaryIRSchema(t *testing.T) {
	path := filepath.Join("..", "..", "schema", "ir.cue")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("unexpected %s: remove it to avoid schema drift", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}
