hw.module @next_delay_fail(in %clk: !seq.clock, in %req: i1, out ack: i1, out __verif_bad: i1) {
  // --- Body ---
  %d1 = seq.compreg %req, %clk : i1
  %d2 = seq.compreg %d1, %clk : i1
  %d3 = seq.compreg %d2, %clk : i1
  %ack = seq.compreg %d3, %clk : i1
  %6 = ltl.delay %ack, 2, 0 : i1
  %8 = seq.from_clock %clk
  %9 = hw.constant -1 : i1
  %10 = ltl.concat %req, %9 : i1, i1
  %11 = ltl.implication %10, %6 : !ltl.sequence, !ltl.sequence
  %12 = ltl.clock %11, posedge %8 : !ltl.property
  %15 = ltl.delay %ack, 1, 1 : i1
  %17 = hw.constant -1 : i1
  %18 = ltl.concat %req, %17 : i1, i1
  %19 = ltl.implication %18, %15 : !ltl.sequence, !ltl.sequence
  %20 = ltl.clock %19, posedge %8 : !ltl.property
  // temporal assertion skipped in IC3 path (type !ltl.property): %12
  // temporal assertion skipped in IC3 path (type !ltl.property): %20
  %21 = hw.constant 0 : i1
  hw.output %ack, %21 : i1, i1
}

