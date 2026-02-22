hw.module @bounded_counter(in %inc: i1, in %clk: !seq.clock, out val: i4) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %inc, %2 : i1
  %5 = hw.constant 1 : i4
  %3 = comb.add %count, %5 : i4
  %6 = comb.mux %0, %3, %count : i4
  %count = seq.compreg %6, %clk : i4
  hw.output %count : i4
}

