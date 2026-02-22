hw.module @test_live(in %req: i1, in %ack: i1, in %clk: !seq.clock) {
  // --- Body ---
  %4 = seq.from_clock %clk
  %5 = ltl.eventually %ack : i1
  %6 = ltl.implication %req, %5 : i1, !ltl.property
  %7 = ltl.clock %6, posedge %4 : !ltl.property
  verif.assert %7 : !ltl.property
  hw.output
}

