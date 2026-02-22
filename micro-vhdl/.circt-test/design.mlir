module attributes {circt.loweringOptions = "fixUpEmptyModules"} {
  hw.module @pass_through(in %clk : i1, in %a : i1, out z : i1) {
    %false = hw.constant false
    %true = hw.constant true
    %0 = comb.xor %a, %true : i1
    verif.assert %0 : i1
    hw.output %false : i1
  }
  verif.formal @pass_through_CheckContract_0 {} {
    %true = hw.constant true
    %0 = verif.symbolic_value : i1
    %1 = comb.xor %0, %true : i1
    verif.assume %1 : i1
    verif.assert %1 : i1
  }
}
