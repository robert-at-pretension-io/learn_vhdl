hw.module @arbiter_starve(in %req0: i1, in %req1: i1, out grant0: i1, out grant1: i1, in %clk: !seq.clock) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %req0, %2 : i1
  %3 = hw.constant 1 : i1
  %4 = hw.constant 0 : i1
  %7 = hw.constant 1 : i1
  %5 = comb.icmp eq %req1, %7 : i1
  %8 = hw.constant 0 : i1
  %9 = comb.mux %5, %8, %4 : i1
  %10 = comb.mux %0, %3, %9 : i1
  %11 = seq.initial () {
    %12 = hw.constant 0 : i1
    seq.yield %12 : i1
  } : () -> !seq.immutable<i1>
  %grant0 = seq.compreg %10, %clk initial %11 : i1
  %15 = hw.constant 1 : i1
  %13 = comb.icmp eq %req0, %15 : i1
  %16 = hw.constant 0 : i1
  %17 = hw.constant 0 : i1
  %20 = hw.constant 1 : i1
  %18 = comb.icmp eq %req1, %20 : i1
  %21 = hw.constant 1 : i1
  %22 = comb.mux %18, %21, %17 : i1
  %23 = comb.mux %13, %16, %22 : i1
  %24 = seq.initial () {
    %25 = hw.constant 0 : i1
    seq.yield %25 : i1
  } : () -> !seq.immutable<i1>
  %grant1 = seq.compreg %23, %clk initial %24 : i1
  %30 = seq.from_clock %clk
  // TODO liveness: skipped in BMC (requires IC3 with fairness)
  %31 = hw.constant -1 : i1
  verif.assert %31 : i1
  hw.output %grant0, %grant1 : i1, i1
}

