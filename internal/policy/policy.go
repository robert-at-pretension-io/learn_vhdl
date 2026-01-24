package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/rego"
)

// Engine evaluates OPA policies against VHDL facts
type Engine struct {
	queries map[string]rego.PreparedEvalQuery
}

// Violation represents a policy violation
type Violation struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

// Result contains the evaluation results
type Result struct {
	Violations []Violation
	Summary    Summary
}

// Summary provides aggregate counts
type Summary struct {
	TotalViolations int `json:"total_violations"`
	Errors          int `json:"errors"`
	Warnings        int `json:"warnings"`
	Info            int `json:"info"`
}

// Input is the data structure passed to OPA
type Input struct {
	Entities      []Entity      `json:"entities"`
	Architectures []Architecture `json:"architectures"`
	Packages      []Package     `json:"packages"`
	Components    []Component   `json:"components"`
	Signals       []Signal      `json:"signals"`
	Ports         []Port        `json:"ports"`
	Dependencies  []Dependency  `json:"dependencies"`
	Symbols       []Symbol      `json:"symbols"`
}

// Simplified types for OPA input (mirrors extractor types)
type Entity struct {
	Name  string `json:"name"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Ports []Port `json:"ports"`
}

type Architecture struct {
	Name       string `json:"name"`
	EntityName string `json:"entity_name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
}

type Package struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type Component struct {
	Name       string `json:"name"`
	EntityRef  string `json:"entity_ref"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	IsInstance bool   `json:"is_instance"`
}

type Signal struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	InEntity string `json:"in_entity"`
}

type Port struct {
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	Line      int    `json:"line"`
	InEntity  string `json:"in_entity"`
}

type Dependency struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Kind     string `json:"kind"`
	Line     int    `json:"line"`
	Resolved bool   `json:"resolved"`
}

type Symbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// New creates a new policy engine, loading policies from the given directory
func New(policyDir string) (*Engine, error) {
	engine := &Engine{
		queries: make(map[string]rego.PreparedEvalQuery),
	}

	// Find all .rego files
	files, err := filepath.Glob(filepath.Join(policyDir, "*.rego"))
	if err != nil {
		return nil, fmt.Errorf("finding policy files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no policy files found in %s", policyDir)
	}

	// Load all policy files
	var modules []func(*rego.Rego)
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f, err)
		}
		modules = append(modules, rego.Module(f, string(content)))
	}

	// Prepare query for all_violations
	opts := append(modules, rego.Query("data.vhdl.compliance.all_violations"))
	query, err := rego.New(opts...).PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("preparing violations query: %w", err)
	}
	engine.queries["violations"] = query

	// Prepare query for summary
	opts = append(modules, rego.Query("data.vhdl.compliance.summary"))
	query, err = rego.New(opts...).PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("preparing summary query: %w", err)
	}
	engine.queries["summary"] = query

	return engine, nil
}

// Evaluate runs the policies against the input data
func (e *Engine) Evaluate(input Input) (*Result, error) {
	ctx := context.Background()

	// Convert input to map for OPA
	inputMap, err := structToMap(input)
	if err != nil {
		return nil, fmt.Errorf("converting input: %w", err)
	}

	result := &Result{}

	// Get violations
	rs, err := e.queries["violations"].Eval(ctx, rego.EvalInput(inputMap))
	if err != nil {
		return nil, fmt.Errorf("evaluating violations: %w", err)
	}

	if len(rs) > 0 && len(rs[0].Expressions) > 0 {
		violations, ok := rs[0].Expressions[0].Value.([]interface{})
		if ok {
			for _, v := range violations {
				vmap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				violation := Violation{
					Rule:     getString(vmap, "rule"),
					Severity: getString(vmap, "severity"),
					File:     getString(vmap, "file"),
					Line:     getInt(vmap, "line"),
					Message:  getString(vmap, "message"),
				}
				result.Violations = append(result.Violations, violation)
			}
		}
	}

	// Get summary
	rs, err = e.queries["summary"].Eval(ctx, rego.EvalInput(inputMap))
	if err != nil {
		return nil, fmt.Errorf("evaluating summary: %w", err)
	}

	if len(rs) > 0 && len(rs[0].Expressions) > 0 {
		smap, ok := rs[0].Expressions[0].Value.(map[string]interface{})
		if ok {
			result.Summary = Summary{
				TotalViolations: getInt(smap, "total_violations"),
				Errors:          getInt(smap, "errors"),
				Warnings:        getInt(smap, "warnings"),
				Info:            getInt(smap, "info"),
			}
		}
	}

	return result, nil
}

// Helper functions
func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case json.Number:
			i, _ := n.Int64()
			return int(i)
		}
	}
	return 0
}
