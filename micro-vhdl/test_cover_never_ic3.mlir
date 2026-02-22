hw.module @Test(in %clk: !seq.clock, in %req: i1, out ack: i1, out __verif_bad: i1) {
  // --- Body ---
  %ack = seq.compreg %req, %clk : i1
  %6 = hw.constant 1 : i1
  %4 = comb.icmp eq %req, %6 : i1
  %3 = comb.and %4, %ack : i1
  %8 = hw.constant 1 : i1
  %2 = comb.icmp eq %3, %8 : i1
  %14 = hw.constant 0 : i1
  %12 = comb.icmp eq %req, %14 : i1
  %11 = comb.and %12, %ack : i1
  %16 = hw.constant 1 : i1
  %10 = comb.icmp eq %11, %16 : i1
  %17 = hw.constant -1 : i1
  %18 = comb.xor %10, %17 : i1
  %19 = hw.constant -1 : i1
  %20 = comb.xor %18, %19 : i1
  hw.output %ack, %20 : i1, i1
}

