module {
  hw.module @hier_top(in %top_in : i1, out top_out : i1, out __verif_bad : i1) {
    %true = hw.constant true
    %0 = comb.xor %top_in, %true : i1
    %true_0 = hw.constant true
    %1 = comb.icmp eq %top_in, %true_0 : i1
    %false = hw.constant false
    %2 = comb.icmp eq %0, %false : i1
    %true_1 = hw.constant true
    %3 = comb.xor %1, %true_1 : i1
    %4 = comb.or %3, %2 : i1
    %true_2 = hw.constant true
    %5 = comb.xor %4, %true_2 : i1
    hw.output %0, %5 : i1, i1
  }
}

