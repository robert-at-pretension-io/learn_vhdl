; VHDL Fact Extraction Queries
; These queries extract structured facts from the Tree-sitter parse tree

; ============================================================================
; ENTITIES
; ============================================================================

; Entity declarations: entity <name> is ... end entity;
(entity_declaration
  name: (identifier) @entity.name) @entity

; ============================================================================
; ARCHITECTURES
; ============================================================================

; Architecture bodies: architecture <name> of <entity> is ... end architecture;
(architecture_body
  name: (identifier) @architecture.name
  entity: (identifier) @architecture.entity) @architecture

; ============================================================================
; PACKAGES
; ============================================================================

; Package declarations: package <name> is ... end package;
(package_declaration
  name: (identifier) @package.name) @package

; Package bodies: package body <name> is ... end package body;
(package_body
  name: (identifier) @package_body.name) @package_body

; ============================================================================
; SIGNALS
; ============================================================================

; Signal declarations: signal <name> : <type>;
(signal_declaration
  name: (identifier) @signal.name) @signal

; ============================================================================
; PORTS (inside entity declarations)
; ============================================================================

; Port parameters in entity/component declarations
(parameter
  (identifier) @port.name) @port

; ============================================================================
; COMPONENTS
; ============================================================================

; Component declarations: component <name> ... end component;
(component_declaration
  name: (identifier) @component.name) @component

; Component instantiations: <label> : <component> port map (...);
(component_instantiation
  label: (identifier) @instance.label
  component: (identifier) @instance.component) @instance

; ============================================================================
; DEPENDENCIES
; ============================================================================

; Library clauses: library <name>;
(library_clause
  (identifier) @library.name) @library

; Use clauses: use <library>.<package>.all;
(use_clause) @use

; ============================================================================
; TYPE DECLARATIONS
; ============================================================================

; Type declarations: type <name> is ...;
(type_declaration
  name: (identifier) @type.name) @type

; Subtype declarations: subtype <name> is ...;
(subtype_declaration
  name: (identifier) @subtype.name) @subtype

; ============================================================================
; CONSTANTS
; ============================================================================

; Constant declarations: constant <name> : <type> := <value>;
(constant_declaration
  name: (identifier) @constant.name) @constant

; ============================================================================
; PROCESSES
; ============================================================================

; Process statements (with optional label)
(process_statement) @process
