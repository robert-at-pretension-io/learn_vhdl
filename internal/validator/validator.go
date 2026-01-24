package validator

import (
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// Validator validates extracted data against the CUE schema
type Validator struct {
	ctx    *cue.Context
	schema cue.Value
}

// New creates a new Validator with the schema loaded from the given path
func New(schemaPath string) (*Validator, error) {
	ctx := cuecontext.New()

	// Load the schema
	instances := load.Instances([]string{schemaPath}, nil)
	if len(instances) == 0 {
		return nil, fmt.Errorf("no CUE instances found at %s", schemaPath)
	}

	inst := instances[0]
	if inst.Err != nil {
		return nil, fmt.Errorf("loading schema: %w", inst.Err)
	}

	schema := ctx.BuildInstance(inst)
	if schema.Err() != nil {
		return nil, fmt.Errorf("building schema: %w", schema.Err())
	}

	return &Validator{
		ctx:    ctx,
		schema: schema,
	}, nil
}

// Validate checks the given data against the schema
func (v *Validator) Validate(data interface{}) error {
	// Marshal data to JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling data: %w", err)
	}

	// Parse JSON into CUE value
	val := v.ctx.CompileBytes(jsonBytes)
	if val.Err() != nil {
		return fmt.Errorf("parsing data: %w", val.Err())
	}

	// Unify with schema
	unified := v.schema.LookupPath(cue.ParsePath("#IR")).Unify(val)
	if unified.Err() != nil {
		return fmt.Errorf("validation failed: %w", unified.Err())
	}

	// Validate
	if err := unified.Validate(); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}

	return nil
}

// ValidateJSON validates JSON bytes directly
func (v *Validator) ValidateJSON(jsonBytes []byte) error {
	val := v.ctx.CompileBytes(jsonBytes)
	if val.Err() != nil {
		return fmt.Errorf("parsing JSON: %w", val.Err())
	}

	unified := v.schema.LookupPath(cue.ParsePath("#IR")).Unify(val)
	if unified.Err() != nil {
		return fmt.Errorf("validation failed: %w", unified.Err())
	}

	return unified.Validate()
}
