module {
  hw.module @invert_wrong_contract(in %clk : i1, in %a : i1, out z : i1) {
    %true = hw.constant true
    %0 = comb.xor %a, %true : i1
    %1 = verif.symbolic_value : i1
    %2 = comb.icmp eq %1, %a : i1
    verif.assume %2 : i1
    hw.output %1 : i1
  }
  verif.formal @invert_wrong_contract_CheckContract_0 {} {
    %0 = verif.symbolic_value : i1
    %true = hw.constant true
    %1 = comb.xor %0, %true : i1
    %2 = comb.icmp eq %1, %0 : i1
    verif.assert %2 : i1
  }
}

