hw.module @my_module(in %clk: !seq.clock, in %a: i1, in %b: i1, out out_sig: i1) {
  // --- Body ---
  %temp = comb.and %a, %b : i1
  %4 = hw.constant 1 : i1
  %2 = comb.icmp eq %a, %4 : i1
  %5 = hw.constant 1 : i1
  %9 = hw.constant 1 : i1
  %7 = comb.icmp eq %b, %9 : i1
  %10 = hw.constant 0 : i1
  %11 = comb.mux %7, %10, %temp : i1
  %12 = comb.mux %2, %5, %11 : i1
  %out_sig = seq.compreg %12, %clk : i1
  hw.output %out_sig : i1
}

