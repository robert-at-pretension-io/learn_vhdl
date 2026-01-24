# Basic VHDL Compliance Rules
package vhdl.compliance

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

# Helper: Check if entity exists in the entities list
entity_exists(name) {
    entity := input.entities[_]
    lower(entity.name) == lower(name)
}

# Rule: Component instantiation must reference a defined component/entity
unresolved_component[violation] {
    comp := input.components[_]
    comp.is_instance == true
    comp.entity_ref != ""
    not component_or_entity_exists(comp.entity_ref)
    violation := {
        "rule": "component_resolved",
        "severity": "warning",
        "file": comp.file,
        "line": comp.line,
        "message": sprintf("Component instance '%s' references undefined '%s'", [comp.name, comp.entity_ref])
    }
}

# Helper: Check if component or entity exists
component_or_entity_exists(name) {
    entity := input.entities[_]
    lower(entity.name) == lower(name)
}

component_or_entity_exists(name) {
    comp := input.components[_]
    comp.is_instance == false
    lower(comp.name) == lower(name)
}

# Rule: Naming convention - entities should use lowercase (info level)
naming_convention[violation] {
    entity := input.entities[_]
    entity.name != lower(entity.name)
    violation := {
        "rule": "naming_convention",
        "severity": "info",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' should use lowercase naming", [entity.name])
    }
}

# Rule: Unresolved dependencies (excluding standard libraries)
unresolved_dependency[violation] {
    dep := input.dependencies[_]
    dep.resolved == false
    dep.kind == "instantiation"
    violation := {
        "rule": "unresolved_dependency",
        "severity": "error",
        "file": dep.source,
        "line": dep.line,
        "message": sprintf("Unresolved dependency: '%s'", [dep.target])
    }
}

# Aggregate all violations
all_violations := missing_ports | orphan_architecture | unresolved_component | naming_convention | unresolved_dependency

# Summary counts
summary := {
    "total_violations": count(all_violations),
    "errors": count([v | v := all_violations[_]; v.severity == "error"]),
    "warnings": count([v | v := all_violations[_]; v.severity == "warning"]),
    "info": count([v | v := all_violations[_]; v.severity == "info"])
}
