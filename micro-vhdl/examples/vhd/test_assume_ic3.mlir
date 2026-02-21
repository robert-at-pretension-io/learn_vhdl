hw.module @handshake(in %clk: !seq.clock, in %req: i1, out ack: i1, out __verif_bad: i1) {
  // --- Body ---
  %ack = seq.compreg %req, %clk : i1
  %4 = seq.compreg %req, %clk : i1
  %5 = hw.constant -1 : i1
  %6 = comb.xor %4, %5 : i1
  %1 = comb.or %6, %req : i1
  %10 = seq.compreg %req, %clk : i1
  %11 = hw.constant -1 : i1
  %12 = comb.xor %10, %11 : i1
  %7 = comb.or %12, %ack : i1
  %13 = hw.constant -1 : i1
  %14 = comb.xor %7, %13 : i1
  hw.output %ack, %14 : i1, i1
}

