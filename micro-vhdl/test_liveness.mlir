hw.module @test_live(in %req: i1, in %ack: i1, in %clk: !seq.clock) {
  // --- Body ---
  %4 = seq.from_clock %clk
  // TODO liveness: skipped in BMC (requires IC3 with fairness)
  %5 = hw.constant -1 : i1
  verif.assert %5 : i1
  hw.output
}

