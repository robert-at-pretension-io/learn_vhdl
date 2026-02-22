hw.module @Test(in %clk: !seq.clock, in %req: i1, out ack: i1) {
  // --- Body ---
  %1 = seq.initial () {
    %2 = hw.constant 0 : i1
    seq.yield %2 : i1
  } : () -> !seq.immutable<i1>
  %ack = seq.compreg %req, %clk initial %1 : i1
  %8 = hw.constant 1 : i1
  %6 = comb.icmp eq %req, %8 : i1
  %5 = comb.and %6, %ack : i1
  %10 = hw.constant 1 : i1
  %4 = comb.icmp eq %5, %10 : i1
  %16 = hw.constant 0 : i1
  %14 = comb.icmp eq %req, %16 : i1
  %13 = comb.and %14, %ack : i1
  %18 = hw.constant 1 : i1
  %12 = comb.icmp eq %13, %18 : i1
  %19 = hw.constant -1 : i1
  %20 = comb.xor %12, %19 : i1
  verif.assert %20 : i1
  hw.output %ack : i1
}

