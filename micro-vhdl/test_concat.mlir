hw.module @test_concat(in %clk : !seq.clock, in %req: i1, in %ack: i1) {
  %0 = seq.from_clock %clk
  %1 = ltl.concat %ack, %ack : i1, i1
  %2 = ltl.implication %req, %1 : i1, !ltl.sequence
  %3 = ltl.clock %2, posedge %0 : !ltl.property
  verif.assert %3 : !ltl.property
}
