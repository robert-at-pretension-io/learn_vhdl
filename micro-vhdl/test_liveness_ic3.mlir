hw.module @test_live(in %req: i1, in %ack: i1, in %clk: !seq.clock, out __verif_bad: i1, out assert_fair: i1) {
  // --- Body ---
  %4 = seq.from_clock %clk
  %5 = hw.constant -1 : i1
  %6 = comb.xor %req, %5 : i1
  %7 = comb.or %6, %ack : i1
  // temporal assertion skipped in IC3 path (type !ltl.property): %7
  %8 = hw.constant 0 : i1
  hw.output %8, %7 : i1, i1
}

