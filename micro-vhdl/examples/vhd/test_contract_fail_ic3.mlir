hw.module @invert_wrong_contract(in %clk: i1, in %a: i1, out z: i1) {
  // --- Body ---
  %1 = hw.constant -1 : i1
  %z = comb.xor %a, %1 : i1
  hw.output %z : i1
}

