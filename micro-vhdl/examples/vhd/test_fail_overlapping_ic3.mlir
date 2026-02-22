hw.module @delay_mismatch(in %req: i1, in %clk: !seq.clock, out ack: i1, out __verif_bad: i1) {
  // --- Body ---
  %ack = seq.compreg %req, %clk : i1
  %4 = hw.constant -1 : i1
  %5 = comb.xor %req, %4 : i1
  %1 = comb.or %5, %ack : i1
  %6 = hw.constant -1 : i1
  %7 = comb.xor %1, %6 : i1
  hw.output %ack, %7 : i1, i1
}

