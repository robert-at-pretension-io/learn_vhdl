hw.module @delay_mismatch(in %req: i1, in %clk: !seq.clock, out ack: i1) {
  // --- Body ---
  %1 = seq.initial () {
    %2 = hw.constant 0 : i1
    seq.yield %2 : i1
  } : () -> !seq.immutable<i1>
  %ack = seq.compreg %req, %clk initial %1 : i1
  %6 = hw.constant -1 : i1
  %7 = comb.xor %req, %6 : i1
  %3 = comb.or %7, %ack : i1
  verif.assert %3 : i1
  hw.output %ack : i1
}

