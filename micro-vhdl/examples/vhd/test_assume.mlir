hw.module @handshake(in %clk: !seq.clock, in %req: i1, out ack: i1) {
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
  %3 = comb.or %10, %req : i1
  %14 = seq.initial () {
    %15 = hw.constant 0 : i1
    seq.yield %15 : i1
  } : () -> !seq.immutable<i1>
  %16 = seq.compreg %req, %clk initial %14 : i1
  %17 = hw.constant -1 : i1
  %18 = comb.xor %16, %17 : i1
  %11 = comb.or %18, %ack : i1
  verif.assume %3 : i1
  verif.assert %11 : i1
  hw.output %ack : i1
}

