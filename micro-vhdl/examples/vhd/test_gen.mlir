hw.module @my_module(in %a: i4, out out_sig: i4) {
  // --- Body ---
  %0 = hw.constant 0 : i32
  %1 = hw.constant 1 : i32
  %2 = hw.constant 2 : i32
  %3 = hw.constant 3 : i32
  hw.output %a : i4
}

