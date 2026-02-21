hw.module @my_module(in %clk: !seq.clock, in %req: i1, out ack: i1, out __verif_bad: i1) {
  // --- Body ---
  %ack = seq.compreg %req, %clk : i1
  %4 = seq.compreg %req, %clk initial  : i1
  %5 = hw.constant -1 : i1
  %6 = comb.xor %4, %5 : i1
  %1 = comb.or %6, %ack : i1
  // TODO liveness: psl assert always {req} -> eventually! {ack} (skipped in BMC)
  %7 = hw.constant true
  %10 = seq.compreg %req, %clk : i1
  %8 = comb.icmp eq %req, %10 : i1
  %11 = comb.and %1, %7 : i1
  %12 = comb.and %11, %8 : i1
  %13 = hw.constant -1 : i1
  %14 = comb.xor %12, %13 : i1
  hw.output %ack, %14 : i1, i1
}

