hw.module @arbiter_starve(in %req0: i1, in %req1: i1, out grant0: i1, out grant1: i1, in %clk: !seq.clock, out __verif_bad: i1, out assert_fair: i1) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %req0, %2 : i1
  %3 = hw.constant 1 : i1
  %4 = hw.constant 0 : i1
  %7 = hw.constant 1 : i1
  %5 = comb.icmp eq %req1, %7 : i1
  %8 = hw.constant 0 : i1
  %9 = comb.mux %5, %8, %4 : i1
  %10 = comb.mux %0, %3, %9 : i1
  %grant0 = seq.compreg %10, %clk : i1
  %13 = hw.constant 1 : i1
  %11 = comb.icmp eq %req0, %13 : i1
  %14 = hw.constant 0 : i1
  %15 = hw.constant 0 : i1
  %18 = hw.constant 1 : i1
  %16 = comb.icmp eq %req1, %18 : i1
  %19 = hw.constant 1 : i1
  %20 = comb.mux %16, %19, %15 : i1
  %21 = comb.mux %11, %14, %20 : i1
  %grant1 = seq.compreg %21, %clk : i1
  %26 = seq.from_clock %clk
  %27 = hw.constant -1 : i1
  %28 = comb.xor %req1, %27 : i1
  %29 = comb.or %28, %grant1 : i1
  // temporal assertion skipped in IC3 path (type !ltl.property): %29
  %30 = hw.constant 0 : i1
  hw.output %grant0, %grant1, %30, %29 : i1, i1, i1, i1
}

