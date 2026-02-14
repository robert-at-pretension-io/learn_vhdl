;; Keywords
[
  "library"
  "use"
  "context"
  "entity"
  "architecture"
  "configuration"
  "package"
  "body"
  "is"
  "of"
  "end"
  "component"
  "generic"
  "port"
  "map"
  "in"
  "out"
  "inout"
  "buffer"
  "linkage"
  "signal"
  "variable"
  "shared"
  "constant"
  "file"
  "type"
  "subtype"
  "alias"
  "attribute"
  "function"
  "procedure"
  "impure"
  "pure"
  "return"
  "begin"
  "process"
  "if"
  "then"
  "elsif"
  "else"
  "case"
  "when"
  "others"
  "for"
  "while"
  "loop"
  "next"
  "exit"
  "wait"
  "on"
  "until"
  "after"
  "transport"
  "inertial"
  "reject"
  "unaffected"
  "block"
  "generate"
  "select"
  "with"
  "record"
  "array"
  "access"
  "protected"
  "units"
  "new"
  "report"
  "severity"
  "assert"
  "assume"
  "cover"
  "restrict"
  "property"
  "sequence"
  "default"
  "clock"
  "always"
  "never"
  "eventually"
  "disconnect"
  "group"
  "view"
  "force"
  "release"
  "parameter"
  "open"
  "to"
  "downto"
  "range"
  "all"
  "null"
] @keyword

;; Operators
[
  "and"
  "or"
  "nand"
  "nor"
  "xor"
  "xnor"
  "not"
  "abs"
  "mod"
  "rem"
  "sll"
  "srl"
  "sla"
  "sra"
  "rol"
  "ror"
] @keyword.operator

[
  "="
  "/="
  "<"
  ">"
  "<="
  ">="
  "?="
  "?/="
  "?<"
  "?>"
  "?<="
  "?>="
  "+"
  "-"
  "*"
  "/"
  "**"
  "&"
  ":="
  "=>"
  "<>"
  "??"
  "<< "
  ">>"
] @operator

;; Punctuation
[
  ";"
  ","
  "."
  ":"
  "'"
  "("
  ")"
  "["
  "]"
  "{"
  "}"
] @punctuation.delimiter

;; Literals
(number) @number
(character_literal) @string
(bit_string_literal) @number
(string_literal) @string
(invalid_bit_string_literal) @error

;; Comments
(comment) @comment
(block_comment) @comment

;; Definitions

(entity_declaration
  name: (identifier) @type)

(architecture_body
  name: (identifier) @type
  entity: (identifier) @type)

(package_declaration
  name: (identifier) @type)

(package_body
  name: (identifier) @type)

(configuration_declaration
  name: (identifier) @type
  entity: (choice (identifier) @type (selected_name) @type))

(component_declaration
  name: (identifier) @type)

(type_declaration
  name: (identifier) @type.definition)

(subtype_declaration
  name: (identifier) @type.definition)

;; Function/Procedure
(function_declaration
  name: (identifier) @function)

(procedure_declaration
  name: (identifier) @function)

(subprogram_instantiation
  name: (identifier) @function)

(alias_declaration
  name: (identifier) @variable)

;; Declarations with lists
(constant_declaration
  names: (identifier_list (identifier) @constant))

(signal_declaration
  names: (identifier_list (identifier) @variable))

(variable_declaration
  names: (identifier_list (identifier) @variable))

(file_declaration
  names: (identifier_list (identifier) @variable))

(element_declaration
  names: (identifier_list (identifier) @variable.member))

(port_clause
  ports: (parameter_list
    (parameter
      names: (identifier_list (identifier) @variable.parameter))))

(generic_map_aspect
  (association_list
    (association_element
      formal: (identifier) @variable.parameter)))

(port_map_aspect
  (association_list
    (association_element
      formal: (identifier) @variable.parameter)))

;; Function calls
(procedure_call_statement
  (identifier) @function)

(concurrent_procedure_call
  (identifier) @function)

(procedure_call_statement
  (selected_name) @function)

(concurrent_procedure_call
  (selected_name) @function)

;; Labels
(process_statement
  label: (identifier) @label)

(block_statement
  label: (identifier) @label)

(generate_statement
  label: (identifier) @label)

(loop_statement
  (identifier) @label)

(case_statement
  label: (identifier) @label)

(if_statement
  label: (identifier) @label)

;; Types usage
(constant_declaration type: (_) @type)
(signal_declaration type: (_) @type)
(variable_declaration type: (_) @type)
(file_declaration type: (_) @type)
(element_declaration type: (_) @type)
(alias_declaration type: (_) @type)
(subtype_declaration indication: (_) @type)
(parameter type: (_) @type)

;; Attributes
(attribute_name
  attribute: (identifier) @keyword.function) 

;; Preprocessor
(protect_directive) @keyword.directive
