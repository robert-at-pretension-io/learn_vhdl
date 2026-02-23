hw.module private @child_module<WIDTH: i32 = 8>(in %in1: i1, out out1: i1) {
  // --- Body ---
  %1 = hw.constant -1 : i1
  %out1 = comb.xor %in1, %1 : i1
  hw.output %out1 : i1
}

hw.module @my_module(in %a: i16, out out_sig: i16) {
  // --- Body ---
  %out_sig = hw.instance "my_inst" @child_module<WIDTH: i32 = 16>(in1: %a: i1) -> (out1: i1)
  hw.output %out_sig : i16
}

