hw.module @sticky_high(in %clk: !seq.clock, in %rst: i1, out s: i1, out __verif_bad: i1) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %rst, %2 : i1
  %3 = hw.constant 1 : i1
  %4 = comb.mux %0, %3, %s_sig : i1
  %s_sig = seq.compreg %4, %clk : i1
  %7 = comb.or %6, %rst : i1
  %6 = seq.compreg %7, %clk : i1
  %9 = hw.constant -1 : i1
  %10 = comb.xor %6, %9 : i1
  %11 = comb.or %10, %rst : i1
  %12 = comb.or %11, %s_sig : i1
  %13 = hw.constant -1 : i1
  %14 = comb.xor %12, %13 : i1
  hw.output %s_sig, %14 : i1, i1
}

