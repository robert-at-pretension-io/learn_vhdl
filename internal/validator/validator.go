package validator

// =============================================================================
// VALIDATOR PHILOSOPHY: CRASH EARLY, CRASH LOUD
// =============================================================================
//
// The CUE validator is the "contract guard" between Go and the Rust policy engine.
//
// WHY THIS EXISTS:
// Without validation, if a field name changes or a type is wrong:
// - The policy engine silently receives `undefined`
// - Rules don't fire
// - You think your code is clean
// - Silent bugs multiply
//
// With validation:
// - Immediate crash with clear error
// - "field 'assigned_signals' not allowed" tells you exactly what's wrong
// - Fix the schema or the code, no guessing
//
// WHEN VALIDATION FAILS:
// 1. DON'T suppress the error or add a workaround
// 2. DON'T add fields to schema.cue without understanding why
// 3. DO trace back: Is this a grammar bug? Extractor bug? Indexer bug?
// 4. DO fix at the source (see AGENTS.md "The Grammar Improvement Cycle")
//
// The validator is the canary in the coal mine. When it complains, listen!
// =============================================================================

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

//go:embed output_schema.cue
var outputSchemaFS embed.FS

//go:embed facts_schema.cue
var factsSchemaFS embed.FS

//go:embed daemon_schema.cue
var daemonSchemaFS embed.FS

// Validator validates extracted data against the CUE schema contract.
// This is the "strict gatekeeper" that prevents silent failures in the policy engine.
// If the data doesn't match the schema, we crash immediately with a
// clear error rather than letting the policy engine silently receive bad data.
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
// This enforces the contract between Go and the policy engine.
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

// ValidateVerificationTag validates a single verification tag against the CUE schema.
func (v *Validator) ValidateVerificationTag(tag interface{}) error {
	jsonBytes, err := json.Marshal(tag)
	if err != nil {
		return fmt.Errorf("marshaling tag to JSON: %w", err)
	}
	dataValue := v.ctx.CompileBytes(jsonBytes)
	if dataValue.Err() != nil {
		return fmt.Errorf("compiling tag as CUE: %w", dataValue.Err())
	}

	tagDef := v.schema.LookupPath(cue.ParsePath("#VerificationTag"))
	if tagDef.Err() != nil {
		return fmt.Errorf("looking up #VerificationTag definition: %w", tagDef.Err())
	}

	unified := tagDef.Unify(dataValue)
	if err := unified.Validate(); err != nil {
		return fmt.Errorf("tag schema validation failed: %w", err)
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

// OutputValidator validates linter output against the output schema
type OutputValidator struct {
	ctx    *cue.Context
	schema cue.Value
}

// NewOutputValidator creates a validator for linter output
func NewOutputValidator() (*OutputValidator, error) {
	ctx := cuecontext.New()

	schemaBytes, err := outputSchemaFS.ReadFile("output_schema.cue")
	if err != nil {
		return nil, fmt.Errorf("loading output schema: %w", err)
	}

	schema := ctx.CompileBytes(schemaBytes)
	if schema.Err() != nil {
		return nil, fmt.Errorf("compiling output schema: %w", schema.Err())
	}

	return &OutputValidator{
		ctx:    ctx,
		schema: schema,
	}, nil
}

// Validate checks that the output data conforms to the output schema
func (v *OutputValidator) Validate(data interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling output to JSON: %w", err)
	}

	dataValue := v.ctx.CompileBytes(jsonBytes)
	if dataValue.Err() != nil {
		return fmt.Errorf("compiling output as CUE: %w", dataValue.Err())
	}

	outputDef := v.schema.LookupPath(cue.ParsePath("#LintOutput"))
	if outputDef.Err() != nil {
		return fmt.Errorf("looking up #LintOutput definition: %w", outputDef.Err())
	}

	unified := outputDef.Unify(dataValue)
	if err := unified.Validate(); err != nil {
		return fmt.Errorf("output schema validation failed: %w", err)
	}

	return nil
}

// FactsValidator validates relational fact tables against the facts schema.
type FactsValidator struct {
	ctx    *cue.Context
	schema cue.Value
}

// NewFactsValidator creates a validator for relational fact tables.
func NewFactsValidator() (*FactsValidator, error) {
	ctx := cuecontext.New()

	schemaBytes, err := factsSchemaFS.ReadFile("facts_schema.cue")
	if err != nil {
		return nil, fmt.Errorf("loading facts schema: %w", err)
	}

	schema := ctx.CompileBytes(schemaBytes)
	if schema.Err() != nil {
		return nil, fmt.Errorf("compiling facts schema: %w", schema.Err())
	}

	return &FactsValidator{
		ctx:    ctx,
		schema: schema,
	}, nil
}

// Validate checks that the fact tables conform to the facts schema.
func (v *FactsValidator) Validate(data interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling facts to JSON: %w", err)
	}

	dataValue := v.ctx.CompileBytes(jsonBytes)
	if dataValue.Err() != nil {
		return fmt.Errorf("compiling facts as CUE: %w", dataValue.Err())
	}

	factsDef := v.schema.LookupPath(cue.ParsePath("#FactTables"))
	if factsDef.Err() != nil {
		return fmt.Errorf("looking up #FactTables definition: %w", factsDef.Err())
	}

	unified := factsDef.Unify(dataValue)
	if err := unified.Validate(); err != nil {
		return fmt.Errorf("facts schema validation failed: %w", err)
	}

	return nil
}

// PolicyDaemonValidator validates vhdl_policyd command/response payloads.
type PolicyDaemonValidator struct {
	ctx    *cue.Context
	schema cue.Value
}

// NewPolicyDaemonValidator creates a validator for policy daemon protocol payloads.
func NewPolicyDaemonValidator() (*PolicyDaemonValidator, error) {
	ctx := cuecontext.New()

	schemaBytes, err := daemonSchemaFS.ReadFile("daemon_schema.cue")
	if err != nil {
		return nil, fmt.Errorf("loading daemon schema: %w", err)
	}

	schema := ctx.CompileBytes(schemaBytes)
	if schema.Err() != nil {
		return nil, fmt.Errorf("compiling daemon schema: %w", schema.Err())
	}

	return &PolicyDaemonValidator{
		ctx:    ctx,
		schema: schema,
	}, nil
}

// ValidateCommandJSON validates a command JSON payload.
func (v *PolicyDaemonValidator) ValidateCommandJSON(jsonBytes []byte) error {
	return v.validateJSON(jsonBytes, "#PolicyDaemonCommand")
}

// ValidateResponseJSON validates a response JSON payload.
func (v *PolicyDaemonValidator) ValidateResponseJSON(jsonBytes []byte) error {
	return v.validateJSON(jsonBytes, "#PolicyDaemonResponse")
}

func (v *PolicyDaemonValidator) validateJSON(jsonBytes []byte, path string) error {
	dataValue := v.ctx.CompileBytes(jsonBytes)
	if dataValue.Err() != nil {
		return fmt.Errorf("compiling JSON as CUE: %w", dataValue.Err())
	}

	def := v.schema.LookupPath(cue.ParsePath(path))
	if def.Err() != nil {
		return fmt.Errorf("looking up %s definition: %w", path, def.Err())
	}

	unified := def.Unify(dataValue)
	if err := unified.Validate(); err != nil {
		return fmt.Errorf("daemon schema validation failed: %w", err)
	}

	return nil
}
