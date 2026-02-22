module {
  verif.formal @bounded_counter_CheckContract_0 {} {
    %0 = verif.symbolic_value : !seq.clock
    %1 = verif.symbolic_value : i1
    %2 = seq.initial() {
      %c0_i4 = hw.constant 0 : i4
      seq.yield %c0_i4 : i4
    } : () -> !seq.immutable<i4>
    %true = hw.constant true
    %c1_i4 = hw.constant 1 : i4
    %3 = comb.icmp eq %1, %true : i1
    %4 = comb.add %count, %c1_i4 : i4
    %5 = comb.mux %3, %4, %count : i4
    %count = seq.compreg %5, %0 initial %2 : i4
    %c-1_i4 = hw.constant -1 : i4
    %6 = comb.icmp ne %count, %c-1_i4 : i4
    verif.assert %6 : i1
  }
}
