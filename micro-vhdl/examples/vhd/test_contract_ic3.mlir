hw.module @Arbiter(in %req0: i1, in %req1: i1, out grant0: i1, out grant1: i1, in %clk: !seq.clock) {
  // --- Body ---
  hw.output %req0, %req1 : i1, i1
}

