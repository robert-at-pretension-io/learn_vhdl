hw.module private @child_module(in %in1: i1, out out1: i1) {
  // --- Body ---
  %1 = hw.constant -1 : i1
  %out1 = comb.xor %in1, %1 : i1
  hw.output %out1 : i1
}

hw.module @my_module(in %clk: !seq.clock, in %a: i1, in %b: i1, out out_sig: i1) {
  // --- Body ---
  %temp = hw.instance "my_inst" @child_module(in1: %a: i1) -> (out1: i1)
  %5 = hw.constant 1 : i1
  %3 = comb.icmp eq %a, %5 : i1
  %6 = hw.constant 1 : i1
  %10 = hw.constant 1 : i1
  %8 = comb.icmp eq %b, %10 : i1
  %11 = hw.constant 0 : i1
  %12 = comb.mux %8, %11, %temp : i1
  %13 = comb.mux %3, %6, %12 : i1
  %out_sig = seq.compreg %13, %clk : i1
  hw.output %out_sig : i1
}

