hw.module @my_module(in %sel: i2, in %a: i1, in %b: i1, in %c: i1, out out_sig: i1, out out2: i1) {
  // --- Body ---
  %3 = hw.constant 0 : i2
  %1 = comb.icmp eq %sel, %3 : i2
  %out2 = comb.mux %1, %a, %b : i1
  %7 = hw.constant 2 : i2
  %8 = comb.icmp eq %sel, %7 : i2
  %10 = comb.mux %8, %b, %c : i1
  %11 = hw.constant 1 : i2
  %12 = comb.icmp eq %sel, %11 : i2
  %out_sig = comb.mux %12, %a, %10 : i1
  hw.output %out_sig, %out2 : i1, i1
}

