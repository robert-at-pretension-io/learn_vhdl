module {
  verif.formal @invert_wrong_contract_CheckContract_0 {} {
    %0 = verif.symbolic_value : i1
    %true = hw.constant true
    %1 = comb.xor %0, %true : i1
    %2 = comb.icmp eq %1, %0 : i1
    verif.assert %2 : i1
  }
}
