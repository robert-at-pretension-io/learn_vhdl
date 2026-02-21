import re

# Update Passes.td
with open("cirt/include/circt/Transforms/Passes.td", "r") as f:
    content = f.read()

# Insert before #endif // CIRCT_TRANSFORMS_PASSES
if "def CheckCombDepth" not in content:
    content = content.replace("#endif // CIRCT_TRANSFORMS_PASSES", """
def CheckCombDepth : Pass<"check-comb-depth", "::mlir::ModuleOp"> {
  let summary = "Check maximum combinational logic depth";
  let description = [{
    Checks if the combinational logic depth exceeds a specified maximum.
  }];
  let constructor = "circt::createCheckCombDepthPass()";
  let options = [
    Option<"maxDepth", "max-depth", "unsigned", "0",
           "Maximum allowed combinational depth">
  ];
  let dependentDialects = ["circt::hw::HWDialect", "circt::comb::CombDialect"];
}

#endif // CIRCT_TRANSFORMS_PASSES
""")
    with open("cirt/include/circt/Transforms/Passes.td", "w") as f:
        f.write(content)


# Update Passes.h
with open("cirt/include/circt/Transforms/Passes.h", "r") as f:
    content = f.read()

decl = "std::unique_ptr<mlir::Pass> createCheckCombDepthPass();\n"
if decl not in content:
    content = content.replace("std::unique_ptr<mlir::Pass> createHierarchicalRunner", decl + "std::unique_ptr<mlir::Pass> createHierarchicalRunner")
    with open("cirt/include/circt/Transforms/Passes.h", "w") as f:
        f.write(content)

# Update CMakeLists.txt
with open("cirt/lib/Transforms/CMakeLists.txt", "r") as f:
    content = f.read()

if "CheckCombDepth.cpp" not in content:
    content = content.replace("  ConvertIndexToUInt.cpp\n", "  ConvertIndexToUInt.cpp\n  CheckCombDepth.cpp\n")
    with open("cirt/lib/Transforms/CMakeLists.txt", "w") as f:
        f.write(content)

