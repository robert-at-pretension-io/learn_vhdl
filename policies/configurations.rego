# Configuration Declaration Rules
# Rules for configuration declarations (entity binding)
package vhdl.configurations

# Rule: Configuration references missing entity
configuration_missing_entity[violation] {
    cfg := input.configurations[_]
    not entity_exists(cfg.entity_name)
    violation := {
        "rule": "configuration_missing_entity",
        "severity": "error",
        "file": cfg.file,
        "line": cfg.line,
        "message": sprintf("Configuration '%s' references missing entity '%s'", [cfg.name, cfg.entity_name])
    }
}

entity_exists(name) {
    e := input.entities[_]
    lower(e.name) == lower(name)
}

violations := configuration_missing_entity
