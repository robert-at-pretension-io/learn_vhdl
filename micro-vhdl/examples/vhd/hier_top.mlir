hw.module @hier_top(in %top_in: i1, out top_out: i1) {
  // --- Body ---
  %top_out = hw.instance "u_child" @hier_child(in1: %top_in: i1) -> (out1: i1)
  %4 = hw.constant 1 : i1
  %2 = comb.icmp eq %top_in, %4 : i1
  %7 = hw.constant 0 : i1
  %5 = comb.icmp eq %top_out, %7 : i1
  %8 = hw.constant -1 : i1
  %9 = comb.xor %2, %8 : i1
  %1 = comb.or %9, %5 : i1
  verif.assert %1 : i1
  hw.output %top_out : i1
}

hw.module private @hier_child(in %in1: i1, out out1: i1) {
  // --- Body ---
  %11 = hw.constant -1 : i1
  %out1 = comb.xor %in1, %11 : i1
  hw.output %out1 : i1
}

