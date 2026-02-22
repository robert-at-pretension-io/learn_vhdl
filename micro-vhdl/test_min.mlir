hw.module @Arbiter(in %req0: i1, in %req1: i1, out grant0: i1, out grant1: i1, in %clk: !seq.clock) {
  %10, %11 = verif.contract %req0, %req1 : i1, i1 {
    %28 = hw.constant 1 : i1
    %26 = comb.icmp eq %req0, %28 : i1
    verif.ensure %26 : i1
  }
  hw.output %10, %11 : i1, i1
}
