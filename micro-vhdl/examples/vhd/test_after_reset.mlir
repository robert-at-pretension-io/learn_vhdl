hw.module @sticky_high(in %clk: !seq.clock, in %rst: i1, out s: i1) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %rst, %2 : i1
  %3 = hw.constant 1 : i1
  %4 = comb.mux %0, %3, %s_sig : i1
  %5 = seq.initial () {
    %6 = hw.constant 0 : i1
    seq.yield %6 : i1
  } : () -> !seq.immutable<i1>
  %s_sig = seq.compreg %4, %clk initial %5 : i1
  %9 = comb.or %8, %rst : i1
  %10 = seq.initial () {
    %11 = hw.constant 0 : i1
    seq.yield %11 : i1
  } : () -> !seq.immutable<i1>
  %8 = seq.compreg %9, %clk initial %10 : i1
  %13 = hw.constant -1 : i1
  %14 = comb.xor %8, %13 : i1
  %15 = comb.or %14, %rst : i1
  %16 = comb.or %15, %s_sig : i1
  verif.assert %16 : i1
  hw.output %s_sig : i1
}

