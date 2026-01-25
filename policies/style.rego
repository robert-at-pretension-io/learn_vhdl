# Style and Maintainability Rules
# Rules for code organization, complexity, and best practices
package vhdl.style

import data.vhdl.helpers

# Rule: Large signal count in entity (complexity smell)
# Entities with too many signals may indicate need for hierarchy
large_entity[violation] {
    entity := input.entities[_]
    port_count := count(entity.ports)
    port_count > 50
    violation := {
        "rule": "large_entity",
        "severity": "info",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' has %d ports - consider breaking into sub-modules", [entity.name, port_count])
    }
}

# Rule: Process missing label (maintainability, debugging)
# Disabled by default - too many unlabeled processes in typical code
process_label_missing[violation] {
    proc := input.processes[_]
    proc.label == ""
    violation := {
        "rule": "process_label_missing",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process at line %d has no label - add 'label: process' for debugging", [proc.line])
    }
}

# Rule: Multiple entities per file (maintainability)
multiple_entities_per_file[violation] {
    file := input.entities[_].file
    entities_in_file := [e | e := input.entities[_]; e.file == file]
    count(entities_in_file) > 1
    first_entity := entities_in_file[0]
    violation := {
        "rule": "multiple_entities_per_file",
        "severity": "info",
        "file": file,
        "line": first_entity.line,
        "message": sprintf("File contains %d entities - consider one entity per file", [count(entities_in_file)])
    }
}

# Rule: Legacy packages (std_logic_arith is non-standard)
legacy_packages[violation] {
    dep := input.dependencies[_]
    contains(lower(dep.target), "std_logic_arith")
    violation := {
        "rule": "legacy_packages",
        "severity": "warning",
        "file": dep.source,
        "line": dep.line,
        "message": "Using std_logic_arith (non-standard) - use ieee.numeric_std instead"
    }
}

legacy_packages[violation] {
    dep := input.dependencies[_]
    contains(lower(dep.target), "std_logic_unsigned")
    violation := {
        "rule": "legacy_packages",
        "severity": "warning",
        "file": dep.source,
        "line": dep.line,
        "message": "Using std_logic_unsigned (non-standard) - use ieee.numeric_std instead"
    }
}

legacy_packages[violation] {
    dep := input.dependencies[_]
    contains(lower(dep.target), "std_logic_signed")
    violation := {
        "rule": "legacy_packages",
        "severity": "warning",
        "file": dep.source,
        "line": dep.line,
        "message": "Using std_logic_signed (non-standard) - use ieee.numeric_std instead"
    }
}

# Rule: Architecture naming convention
# Common conventions: rtl, behavioral, structural, testbench
# Disabled by default - project-specific
architecture_naming_convention[violation] {
    arch := input.architectures[_]
    not helpers.is_standard_arch_name(arch.name)
    violation := {
        "rule": "architecture_naming_convention",
        "severity": "info",
        "file": arch.file,
        "line": arch.line,
        "message": sprintf("Architecture '%s' uses non-standard name - consider rtl, behavioral, or structural", [arch.name])
    }
}

# Rule: Empty architecture (no signals, instances, or processes)
# Disabled by default - too noisy
empty_architecture[violation] {
    arch := input.architectures[_]
    signals_in_arch := [s | s := input.signals[_]; s.in_entity == arch.entity_name]
    instances_in_arch := [i | i := input.instances[_]; i.in_arch == arch.name]
    processes_in_arch := [p | p := input.processes[_]; p.in_arch == arch.name]
    count(signals_in_arch) == 0
    count(instances_in_arch) == 0
    count(processes_in_arch) == 0
    violation := {
        "rule": "empty_architecture",
        "severity": "warning",
        "file": arch.file,
        "line": arch.line,
        "message": sprintf("Architecture '%s' is empty (no signals, instances, or processes)", [arch.name])
    }
}

# Aggregate style violations (enabled by default)
violations := large_entity | multiple_entities_per_file | legacy_packages

# Optional violations (disabled by default)
optional_violations := process_label_missing | architecture_naming_convention | empty_architecture
