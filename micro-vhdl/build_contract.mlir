hw.module @Arbiter(in %req0: i1, in %req1: i1, out grant0: i1, out grant1: i1, in %clk: !seq.clock) {
  // --- Body ---
  %0, %1 = verif.contract %req0, %req1 : i1, i1 {
    %7 = hw.constant 1 : i1
    %5 = comb.icmp eq %req0, %7 : i1
    %10 = hw.constant 1 : i1
    %8 = comb.icmp eq %req1, %10 : i1
    %4 = comb.and %5, %8 : i1
    %11 = hw.constant -1 : i1
    %2 = comb.xor %4, %11 : i1
    verif.require %2 : i1
    %17 = hw.constant 1 : i1
    %15 = comb.icmp eq %0, %17 : i1
    %20 = hw.constant 1 : i1
    %18 = comb.icmp eq %1, %20 : i1
    %14 = comb.and %15, %18 : i1
    %21 = hw.constant -1 : i1
    %12 = comb.xor %14, %21 : i1
    verif.ensure %12 : i1
  }
  hw.output %0, %1 : i1, i1
}

