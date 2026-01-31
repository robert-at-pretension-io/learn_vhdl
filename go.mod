module github.com/robert-at-pretension-io/vhdl-lint

go 1.24.6

toolchain go1.24.12

require (
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82
	github.com/tree-sitter/tree-sitter-vhdl v0.0.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/tree-sitter/tree-sitter-vhdl => ./tree-sitter-vhdl/bindings/go

require (
	cuelang.org/go v0.15.3
	github.com/cockroachdb/apd/v3 v3.2.1 // indirect
	github.com/emicklei/proto v1.14.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/protocolbuffers/txtpbfmt v0.0.0-20251016062345-16587c79cd91 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
