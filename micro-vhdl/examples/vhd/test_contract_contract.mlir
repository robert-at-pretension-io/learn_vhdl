module {
  hw.module @pass_through(in %clk : i1, in %a : i1, out z : i1) {
    %0 = verif.symbolic_value : i1
    %false = hw.constant false
    %1 = comb.icmp eq %a, %false : i1
    verif.assert %1 : i1
    %false_0 = hw.constant false
    %2 = comb.icmp eq %0, %false_0 : i1
    verif.assume %2 : i1
    hw.output %0 : i1
  }
  verif.formal @pass_through_CheckContract_0 {} {
    %0 = verif.symbolic_value : i1
    %false = hw.constant false
    %1 = comb.icmp eq %0, %false : i1
    verif.assume %1 : i1
    %false_0 = hw.constant false
    %2 = comb.icmp eq %0, %false_0 : i1
    verif.assert %2 : i1
  }
}

