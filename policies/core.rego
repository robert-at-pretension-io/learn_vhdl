# Core VHDL Structural Rules
# Rules for basic design unit integrity: entities, architectures, components, dependencies
package vhdl.core

import data.vhdl.helpers

# Rule: Every entity should have at least one port
missing_ports[violation] {
    entity := input.entities[_]
    count(entity.ports) == 0
    not helpers.is_testbench_name(entity.name)
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
    lower(entity.name) == base_entity_name(name)
}

component_or_entity_exists(name) {
    comp := input.components[_]
    comp.is_instance == false
    lower(comp.name) == base_entity_name(name)
}

base_entity_name(name) := lower(name) {
    not contains(name, ".")
}

base_entity_name(name) := lower(parts[count(parts) - 1]) {
    parts := split(name, ".")
    count(parts) > 1
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

# Rule: Incomplete case statement (potential latch)
# A case statement without "when others =>" can infer a latch in combinational logic
# This is the #1 bug in FPGA design
potential_latch[violation] {
    cs := input.case_statements[_]
    cs.has_others == false
    violation := {
        "rule": "potential_latch",
        "severity": "warning",
        "file": cs.file,
        "line": cs.line,
        "message": sprintf("Case statement on '%s' missing 'when others =>' (potential latch in process '%s')", [cs.expression, cs.in_process])
    }
}

# Rule: Entity with no architecture (incomplete design unit)
entity_without_arch[violation] {
    entity := input.entities[_]
    not has_architecture(entity.name)
    violation := {
        "rule": "entity_without_arch",
        "severity": "warning",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' has no architecture defined", [entity.name])
    }
}

# Helper: Check if entity has an architecture
has_architecture(entity_name) {
    arch := input.architectures[_]
    lower(arch.entity_name) == lower(entity_name)
}

# Aggregate core violations
violations := missing_ports | orphan_architecture | unresolved_component | unresolved_dependency | potential_latch | entity_without_arch
