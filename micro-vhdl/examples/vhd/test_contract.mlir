hw.module @pass_through(in %clk: i1, in %a: i1, out z: i1) {
  // --- Body ---
  %0 = verif.contract %a : i1 {
    %3 = hw.constant 0 : i1
    %1 = comb.icmp eq %a, %3 : i1
    verif.require %1 : i1
    %6 = hw.constant 0 : i1
    %4 = comb.icmp eq %0, %6 : i1
    verif.ensure %4 : i1
  }
  hw.output %0 : i1
}

