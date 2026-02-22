hw.module @pass_through(in %clk: i1, in %a: i1, out z: i1) {
  // --- Body ---
  hw.output %a : i1
}

