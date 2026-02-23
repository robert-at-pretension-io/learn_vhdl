module {
  verif.formal @Arbiter_CheckContract_0 {} {
    %0 = verif.symbolic_value : i1
    %1 = verif.symbolic_value : i1
    %true = hw.constant true
    %2 = comb.icmp eq %0, %true : i1
    %true_0 = hw.constant true
    %3 = comb.icmp eq %1, %true_0 : i1
    %4 = comb.and %2, %3 : i1
    %true_1 = hw.constant true
    %5 = comb.xor %4, %true_1 : i1
    verif.assume %5 : i1
    %true_2 = hw.constant true
    %6 = comb.icmp eq %0, %true_2 : i1
    %true_3 = hw.constant true
    %7 = comb.icmp eq %1, %true_3 : i1
    %8 = comb.and %6, %7 : i1
    %true_4 = hw.constant true
    %9 = comb.xor %8, %true_4 : i1
    verif.assert %9 : i1
  }
}
