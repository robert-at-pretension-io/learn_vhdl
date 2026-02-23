hw.module @Dummy(in %clk: !seq.clock) {
  // --- Body ---
  verif.formal @CheckMult {} {
    %a = verif.symbolic_value : i32
    %b = verif.symbolic_value : i32
    %2 = comb.mul %a, %b : i32
    %5 = comb.mul %b, %a : i32
    %1 = comb.icmp eq %2, %5 : i32
    verif.assert %1 : i1
  }
  hw.output
}

