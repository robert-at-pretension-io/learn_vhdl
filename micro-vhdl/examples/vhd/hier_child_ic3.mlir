hw.module private @hier_child(in %in1: i1, out out1: i1) {
  // --- Body ---
  %1 = hw.constant -1 : i1
  %out1 = comb.xor %in1, %1 : i1
  hw.output %out1 : i1
}

hw.module @hier_top(in %top_in: i1, out top_out: i1, out __verif_bad: i1) {
  // --- Body ---
  %top_out = hw.instance "u_child" @hier_child(in1: %top_in: i1) -> (out1: i1)
  %6 = hw.constant 1 : i1
  %4 = comb.icmp eq %top_in, %6 : i1
  %9 = hw.constant 0 : i1
  %7 = comb.icmp eq %top_out, %9 : i1
  %10 = hw.constant -1 : i1
  %11 = comb.xor %4, %10 : i1
  %3 = comb.or %11, %7 : i1
  %12 = hw.constant -1 : i1
  %13 = comb.xor %3, %12 : i1
  hw.output %top_out, %13 : i1, i1
}

