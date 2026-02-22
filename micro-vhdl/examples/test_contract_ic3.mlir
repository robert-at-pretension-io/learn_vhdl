hw.module @Arbiter(in %req0: i1, in %req1: i1, out grant0: i1, out grant1: i1, in %clk: !seq.clock) {
  // --- Body ---
  %grant0 = seq.compreg %req0, %clk : i1
  %5 = hw.constant -1 : i1
  %3 = comb.xor %req0, %5 : i1
  %1 = comb.and %req1, %3 : i1
  %grant1 = seq.compreg %1, %clk : i1
  hw.output %grant0, %grant1 : i1, i1
}

