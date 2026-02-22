hw.module @test_fail_counter(in %clk: !seq.clock, in %rst: i1, in %req: i1, out cnt_out: i4, out __verif_bad: i1) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %rst, %2 : i1
  %3 = hw.constant 0 : i4
  %6 = hw.constant 1 : i1
  %4 = comb.icmp eq %req, %6 : i1
  %9 = hw.constant 1 : i4
  %7 = comb.add %cnt, %9 : i4
  %10 = comb.mux %4, %7, %cnt : i4
  %11 = comb.mux %0, %3, %10 : i4
  %cnt = seq.compreg %11, %clk : i4
  %15 = hw.constant 3 : i4
  %13 = comb.icmp ne %cnt, %15 : i4
  %16 = hw.constant -1 : i1
  %17 = comb.xor %13, %16 : i1
  hw.output %cnt, %17 : i4, i1
}

