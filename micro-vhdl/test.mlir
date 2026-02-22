module {
  hw.module @Arbiter(in %req0 : i1, in %req1 : i1, out grant0 : i1, out grant1 : i1, in %clk : !seq.clock) {
    %true = hw.constant true
    %0 = verif.symbolic_value : i1
    %1 = verif.symbolic_value : i1
    %2 = comb.and %req0, %req1 : i1
    %3 = comb.xor %2, %true : i1
    verif.assert %3 : i1
    %4 = comb.and %0, %1 : i1
    %5 = comb.xor %4, %true : i1
    verif.assume %5 : i1
    hw.output %0, %1 : i1, i1
  }
  verif.formal @Arbiter_CheckContract_0 {} {
    %true = hw.constant true
    %0 = verif.symbolic_value : i1
    %1 = verif.symbolic_value : i1
    %2 = verif.symbolic_value : !seq.clock
    %3 = seq.initial() {
      %false = hw.constant false
      seq.yield %false : i1
    } : () -> !seq.immutable<i1>
    %4 = comb.xor %0, %true : i1
    %grant0 = seq.compreg %0, %2 initial %3 : i1  
    %5 = comb.and %1, %4 : i1
    %grant1 = seq.compreg %5, %2 initial %3 : i1  
    %6 = comb.and %0, %1 : i1
    %7 = comb.xor %6, %true : i1
    verif.assume %7 : i1
    %8 = comb.and %grant0, %grant1 : i1
    %9 = comb.xor %8, %true : i1
    verif.assert %9 : i1
  }
}

