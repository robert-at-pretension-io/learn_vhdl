hw.module @my_module(in %clk: !seq.clock, in %req: i1, out ack: i1) {
  // --- Body ---
  %1 = seq.initial () {
    %2 = hw.constant 0 : i1
    seq.yield %2 : i1
  } : () -> !seq.immutable<i1>
  %ack = seq.compreg %req, %clk initial %1 : i1
  %6 = seq.initial () {
    %7 = hw.constant 0 : i1
    seq.yield %7 : i1
  } : () -> !seq.immutable<i1>
  %8 = seq.compreg %req, %clk initial %6 : i1
  %9 = hw.constant -1 : i1
  %10 = comb.xor %8, %9 : i1
  %3 = comb.or %10, %ack : i1
  %14 = seq.from_clock %clk
  // TODO liveness: skipped in BMC (requires IC3 with fairness)
  %15 = hw.constant -1 : i1
  %18 = seq.initial () {
    %19 = hw.constant 0 : i1
    seq.yield %19 : i1
  } : () -> !seq.immutable<i1>
  %20 = seq.compreg %req, %clk initial %18 : i1
  %16 = comb.icmp eq %req, %20 : i1
  %21 = comb.and %3, %15 : i1
  %22 = comb.and %21, %16 : i1
  verif.assert %22 : i1
  hw.output %ack : i1
}

