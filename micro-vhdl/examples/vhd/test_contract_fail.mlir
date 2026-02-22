hw.module @invert_wrong_contract(in %clk: i1, in %a: i1, out z: i1) {
  // --- Body ---
  %1 = hw.constant -1 : i1
  %z = comb.xor %a, %1 : i1
  %2 = verif.contract %z : i1 {
    %3 = comb.icmp eq %2, %a : i1
    verif.ensure %3 : i1
  }
  hw.output %2 : i1
}

