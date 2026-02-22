hw.module @Arbiter(in %req0: i1, in %req1: i1, out grant0: i1, out grant1: i1, in %clk: !seq.clock) {
  // --- Body ---
  %1 = seq.initial () {
    %2 = hw.constant 0 : i1
    seq.yield %2 : i1
  } : () -> !seq.immutable<i1>
  %grant0 = seq.compreg %req0, %clk initial %1 : i1
  %7 = hw.constant -1 : i1
  %5 = comb.xor %req0, %7 : i1
  %3 = comb.and %req1, %5 : i1
  %8 = seq.initial () {
    %9 = hw.constant 0 : i1
    seq.yield %9 : i1
  } : () -> !seq.immutable<i1>
  %grant1 = seq.compreg %3, %clk initial %8 : i1
  %10, %11 = verif.contract %grant0, %grant1 : i1, i1 {
    %18 = hw.constant 1 : i1
    %16 = comb.icmp eq %req0, %18 : i1
    %15 = comb.and %16, %req1 : i1
    %20 = hw.constant 1 : i1
    %14 = comb.icmp eq %15, %20 : i1
    %21 = hw.constant -1 : i1
    %12 = comb.xor %14, %21 : i1
    verif.require %12 : i1
    %28 = hw.constant 1 : i1
    %26 = comb.icmp eq %10, %28 : i1
    %25 = comb.and %26, %11 : i1
    %30 = hw.constant 1 : i1
    %24 = comb.icmp eq %25, %30 : i1
    %31 = hw.constant -1 : i1
    %22 = comb.xor %24, %31 : i1
    verif.ensure %22 : i1
  }
  hw.output %10, %11 : i1, i1
}

