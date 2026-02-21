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
  // TODO liveness: psl assert always {req} -> eventually! {ack} (skipped in BMC)
  %11 = hw.constant true
  %14 = seq.initial () {
    %15 = hw.constant 0 : i1
    seq.yield %15 : i1
  } : () -> !seq.immutable<i1>
  %16 = seq.compreg %req, %clk initial %14 : i1
  %12 = comb.icmp eq %req, %16 : i1
  %17 = comb.and %3, %11 : i1
  %18 = comb.and %17, %12 : i1
  verif.assert %18 : i1
  hw.output %ack : i1
}

