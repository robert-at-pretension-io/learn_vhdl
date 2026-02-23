hw.module @my_module(in %clk: !seq.clock, in %req: i1, out ack: i1, out __verif_bad: i1, out assert_fair: i1) {
  // --- Body ---
  %ack = seq.compreg %req, %clk : i1
  %4 = seq.compreg %req, %clk : i1
  %5 = hw.constant -1 : i1
  %6 = comb.xor %4, %5 : i1
  %1 = comb.or %6, %ack : i1
  %10 = seq.from_clock %clk
  %11 = hw.constant -1 : i1
  %12 = comb.xor %req, %11 : i1
  %13 = comb.or %12, %ack : i1
  %16 = seq.compreg %req, %clk : i1
  %14 = comb.icmp eq %req, %16 : i1
  %17 = comb.and %1, %14 : i1
  %18 = hw.constant -1 : i1
  %19 = comb.xor %17, %18 : i1
  // temporal assertion skipped in IC3 path (type !ltl.property): %13
  hw.output %ack, %19, %13 : i1, i1, i1
}

