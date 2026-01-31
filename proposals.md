# Extractor Upgrade Proposals

Goal: better align extractor with grammar and expose richer abstractions for the Rust rule engine.

1. **Structured generics (entity/component)**
   - Extract generics as first-class facts with name, type, class, default, and scope (entity/component).
   - Motivates: type/width conformance rules, missing/extra generics, default usage.

2. **Component declarations with ports/generics**
   - Extract declared component port/generic interfaces (not just the name).
   - Motivates: validate instantiations against component/interface declarations.

3. **Structured use/library/context clauses**
   - Capture each use clause item (qualified name + `all`), library names, and context references.
   - Motivates: dependency visibility/scope rules and package resolution checks.

4. **Structured association elements for instances**
   - Preserve association element structure (positional vs named, formal, actual, actual kind/base/full path).
   - Motivates: port/generic conformance, unconnected `open`, and dataflow checks.

5. **Process-local details + call facts**
   - Extract process-local variable declarations, wait statements, and procedure calls.
   - Extract function calls (name + arguments) from expression nodes.
   - Motivates: “combinational process with wait”, side-effecting calls, and call/driver analysis.
