module {
  hw.module @Arbiter(in %req0 : i1, in %req1 : i1, out grant0 : i1, out grant1 : i1, in %clk : !seq.clock) {
    %0 = seq.initial() {
      %false = hw.constant false
      seq.yield %false : i1
    } : () -> !seq.immutable<i1>
    %grant0 = seq.compreg %req0, %clk initial %0 : i1  
    %true = hw.constant true
    %1 = comb.xor %req0, %true : i1
    %2 = comb.and %req1, %1 : i1
    %3 = seq.initial() {
      %false = hw.constant false
      seq.yield %false : i1
    } : () -> !seq.immutable<i1>
    %grant1 = seq.compreg %2, %clk initial %3 : i1  
    %4 = verif.symbolic_value : i1
    %5 = verif.symbolic_value : i1
    %true_0 = hw.constant true
    %6 = comb.icmp eq %req0, %true_0 : i1
    %7 = comb.and %6, %req1 : i1
    %true_1 = hw.constant true
    %8 = comb.icmp eq %7, %true_1 : i1
    %true_2 = hw.constant true
    %9 = comb.xor %8, %true_2 : i1
    verif.assert %9 : i1
    %true_3 = hw.constant true
    %10 = comb.icmp eq %4, %true_3 : i1
    %11 = comb.and %10, %5 : i1
    %true_4 = hw.constant true
    %12 = comb.icmp eq %11, %true_4 : i1
    %true_5 = hw.constant true
    %13 = comb.xor %12, %true_5 : i1
    verif.assume %13 : i1
    hw.output %4, %5 : i1, i1
  }
  verif.formal @Arbiter_CheckContract_0 {} {
    %0 = verif.symbolic_value : i1
    %1 = verif.symbolic_value : i1
    %2 = verif.symbolic_value : !seq.clock
    %3 = seq.initial() {
      %false = hw.constant false
      seq.yield %false : i1
    } : () -> !seq.immutable<i1>
    %4 = seq.initial() {
      %false = hw.constant false
      seq.yield %false : i1
    } : () -> !seq.immutable<i1>
    %true = hw.constant true
    %5 = comb.xor %0, %true : i1
    %grant0 = seq.compreg %0, %2 initial %3 : i1  
    %6 = comb.and %1, %5 : i1
    %grant1 = seq.compreg %6, %2 initial %4 : i1  
    %true_0 = hw.constant true
    %7 = comb.icmp eq %0, %true_0 : i1
    %8 = comb.and %7, %1 : i1
    %true_1 = hw.constant true
    %9 = comb.icmp eq %8, %true_1 : i1
    %true_2 = hw.constant true
    %10 = comb.xor %9, %true_2 : i1
    verif.assume %10 : i1
    %true_3 = hw.constant true
    %11 = comb.icmp eq %grant0, %true_3 : i1
    %12 = comb.and %11, %grant1 : i1
    %true_4 = hw.constant true
    %13 = comb.icmp eq %12, %true_4 : i1
    %true_5 = hw.constant true
    %14 = comb.xor %13, %true_5 : i1
    verif.assert %14 : i1
  }
}

