# Subprogram Rules
# Rules for function/procedure parameter legality
package vhdl.subprograms

# Rule: Function parameters must be mode "in" (or empty)
function_param_invalid_mode[violation] {
    fn := input.functions[_]
    param := fn.parameters[_]
    param.direction != ""
    lower(param.direction) != "in"
    violation := {
        "rule": "function_param_invalid_mode",
        "severity": "error",
        "file": fn.file,
        "line": param.line,
        "message": sprintf("Function '%s' parameter '%s' has invalid mode '%s' (only 'in' allowed)", [fn.name, param.name, param.direction])
    }
}

# Rule: Procedure parameters cannot use "buffer" or "linkage"
procedure_param_invalid_mode[violation] {
    pr := input.procedures[_]
    param := pr.parameters[_]
    invalid_modes := {"buffer", "linkage"}
    invalid_modes[lower(param.direction)]
    violation := {
        "rule": "procedure_param_invalid_mode",
        "severity": "error",
        "file": pr.file,
        "line": param.line,
        "message": sprintf("Procedure '%s' parameter '%s' has invalid mode '%s'", [pr.name, param.name, param.direction])
    }
}

violations := function_param_invalid_mode | procedure_param_invalid_mode
