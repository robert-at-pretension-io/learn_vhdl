package validator

import (
	"embed"
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
)

//go:embed schema.cue
var schemaFS embed.FS

// Validator validates extracted data against the CUE schema contract.
// This is the "strict gatekeeper" that prevents silent failures in OPA.
// If the data doesn't match the schema, we crash immediately with a
// clear error rather than letting OPA silently receive bad data.
type Validator struct {
	ctx    *cue.Context
	schema cue.Value
}

// New creates a new Validator with the embedded CUE schema
func New() (*Validator, error) {
	ctx := cuecontext.New()

	// Load the embedded schema
	schemaBytes, err := schemaFS.ReadFile("schema.cue")
	if err != nil {
		return nil, fmt.Errorf("loading embedded schema: %w", err)
	}

	schema := ctx.CompileBytes(schemaBytes)
	if schema.Err() != nil {
		return nil, fmt.Errorf("compiling schema: %w", schema.Err())
	}

	return &Validator{
		ctx:    ctx,
		schema: schema,
	}, nil
}

// Validate checks that the input data conforms to the CUE schema.
// This enforces the contract between Go and OPA.
// Returns nil if valid, or a detailed error explaining what failed.
func (v *Validator) Validate(data interface{}) error {
	// Marshal the Go data to JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling data to JSON: %w", err)
	}

	// Compile the JSON data as a CUE value
	dataValue := v.ctx.CompileBytes(jsonBytes)
	if dataValue.Err() != nil {
		return fmt.Errorf("compiling data as CUE: %w", dataValue.Err())
	}

	// Get the #Input definition from the schema
	inputDef := v.schema.LookupPath(cue.ParsePath("#Input"))
	if inputDef.Err() != nil {
		return fmt.Errorf("looking up #Input definition: %w", inputDef.Err())
	}

	// Unify the data with the schema (this is CUE's type checking)
	unified := inputDef.Unify(dataValue)
	if err := unified.Validate(); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}

// ValidateJSON validates JSON bytes directly against the schema
func (v *Validator) ValidateJSON(jsonBytes []byte) error {
	dataValue := v.ctx.CompileBytes(jsonBytes)
	if dataValue.Err() != nil {
		return fmt.Errorf("compiling JSON as CUE: %w", dataValue.Err())
	}

	inputDef := v.schema.LookupPath(cue.ParsePath("#Input"))
	if inputDef.Err() != nil {
		return fmt.Errorf("looking up #Input definition: %w", inputDef.Err())
	}

	unified := inputDef.Unify(dataValue)
	if err := unified.Validate(); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}

// ValidationErrors returns detailed information about all validation errors
func (v *Validator) ValidationErrors(data interface{}) []string {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return []string{fmt.Sprintf("marshal error: %v", err)}
	}

	dataValue := v.ctx.CompileBytes(jsonBytes)
	if dataValue.Err() != nil {
		return []string{fmt.Sprintf("compile error: %v", dataValue.Err())}
	}

	inputDef := v.schema.LookupPath(cue.ParsePath("#Input"))
	if inputDef.Err() != nil {
		return []string{fmt.Sprintf("schema lookup error: %v", inputDef.Err())}
	}

	unified := inputDef.Unify(dataValue)
	err = unified.Validate()
	if err == nil {
		return nil
	}

	// Extract all errors
	var errs []string
	for _, e := range errors.Errors(err) {
		errs = append(errs, e.Error())
	}
	return errs
}
