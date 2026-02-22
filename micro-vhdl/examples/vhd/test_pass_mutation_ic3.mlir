hw.module @test_pass_mutation(in %clk: !seq.clock, in %rst: i1, in %a: i4, in %b: i4, out c: i4, out __verif_bad: i1) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %rst, %2 : i1
  %3 = hw.constant 0 : i4
  %4 = comb.add %a, %b : i4
  %7 = comb.mux %0, %3, %4 : i4
  %c = seq.compreg %7, %clk : i4
  %14 = hw.constant 0 : i1
  %12 = comb.icmp eq %rst, %14 : i1
  %17 = hw.constant 2 : i4
  %15 = comb.icmp eq %a, %17 : i4
  %11 = comb.and %12, %15 : i1
  %20 = hw.constant 3 : i4
  %18 = comb.icmp eq %b, %20 : i4
  %10 = comb.and %11, %18 : i1
  %24 = hw.constant 5 : i4
  %22 = comb.icmp eq %c, %24 : i4
  %21 = ltl.delay %22, 1, 0 : i1
  %25 = seq.from_clock %clk
  %26 = ltl.implication %10, %21 : i1, !ltl.sequence
  %27 = ltl.clock %26, posedge %25 : !ltl.property
  // temporal assertion skipped in IC3 path (type !ltl.property): %27
  %28 = hw.constant 0 : i1
  hw.output %c, %28 : i4, i1
}

