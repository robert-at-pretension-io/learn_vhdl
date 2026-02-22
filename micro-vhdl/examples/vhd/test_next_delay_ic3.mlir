hw.module @next_delay_demo(in %clk: !seq.clock, in %req: i1, out ack: i1, out __verif_bad: i1) {
  // --- Body ---
  %d1 = seq.compreg %req, %clk : i1
  %d2 = seq.compreg %d1, %clk : i1
  %d3 = seq.compreg %d2, %clk : i1
  %ack = seq.compreg %d3, %clk : i1
  %7 = seq.from_clock %clk
  %8 = ltl.delay %ack, 4, 0 : i1
  %9 = ltl.implication %req, %8 : i1, !ltl.sequence
  %10 = ltl.clock %9, posedge %7 : !ltl.property
  %14 = ltl.delay %ack, 2, 3 : i1
  %15 = ltl.implication %req, %14 : i1, !ltl.sequence
  %16 = ltl.clock %15, posedge %7 : !ltl.property
  // temporal assertion skipped in IC3 path (type !ltl.property): %10
  // temporal assertion skipped in IC3 path (type !ltl.property): %16
  %17 = hw.constant 0 : i1
  hw.output %ack, %17 : i1, i1
}

