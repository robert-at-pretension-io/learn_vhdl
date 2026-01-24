# Basic VHDL Compliance Rules
package vhdl.compliance

import future.keywords.in
import future.keywords.every

# Rule: Every entity should have at least one port
missing_ports[violation] {
    entity := input.entities[_]
    count(entity.ports) == 0
    violation := {
        "rule": "entity_has_ports",
        "severity": "warning",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' has no ports defined", [entity.name])
    }
}

# Rule: Architecture must reference a defined entity
orphan_architecture[violation] {
    arch := input.architectures[_]
    not entity_exists(arch.entity_name)
    violation := {
        "rule": "architecture_has_entity",
        "severity": "error",
        "file": arch.file,
        "line": arch.line,
        "message": sprintf("Architecture '%s' references undefined entity '%s'", [arch.name, arch.entity_name])
    }
}

# Helper: Check if entity exists in symbol table
entity_exists(name) {
    symbol := input.symbols[_]
    symbol.kind == "entity"
    lower(symbol.name) == sprintf("work.%s", [lower(name)])
}

# Rule: Component instantiation must reference a defined component/entity
unresolved_component[violation] {
    dep := input.dependencies[_]
    dep.kind == "instantiation"
    dep.resolved == false
    violation := {
        "rule": "component_resolved",
        "severity": "error",
        "file": dep.source,
        "line": dep.line,
        "message": sprintf("Component '%s' is not defined", [dep.target])
    }
}

# Rule: Naming convention - entities should be lowercase with underscores
naming_convention[violation] {
    entity := input.entities[_]
    not valid_name(entity.name)
    violation := {
        "rule": "naming_convention",
        "severity": "info",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' should use lowercase_with_underscores naming", [entity.name])
    }
}

# Helper: Valid VHDL name (lowercase with underscores)
valid_name(name) {
    re_match(`^[a-z][a-z0-9_]*$`, name)
}

# Rule: Detect potential clock domain crossing (signals used across different clock domains)
# This is a placeholder - real implementation would need clock domain analysis
clock_domain_crossing[violation] {
    # Placeholder: Would need clock domain information
    false
    violation := {
        "rule": "clock_domain_crossing",
        "severity": "error",
        "message": "Potential clock domain crossing detected"
    }
}

# Aggregate all violations
all_violations := missing_ports | orphan_architecture | unresolved_component | naming_convention

# Summary
summary := {
    "total_violations": count(all_violations),
    "errors": count([v | v := all_violations[_]; v.severity == "error"]),
    "warnings": count([v | v := all_violations[_]; v.severity == "warning"]),
    "info": count([v | v := all_violations[_]; v.severity == "info"])
}
