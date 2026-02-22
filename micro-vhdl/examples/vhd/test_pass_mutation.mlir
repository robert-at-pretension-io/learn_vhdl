hw.module @test_pass_mutation(in %clk: !seq.clock, in %rst: i1, in %a: i4, in %b: i4, out c: i4) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %rst, %2 : i1
  %3 = hw.constant 0 : i4
  %4 = comb.add %a, %b : i4
  %7 = comb.mux %0, %3, %4 : i4
  %8 = seq.initial () {
    %9 = hw.constant 0 : i4
    seq.yield %9 : i4
  } : () -> !seq.immutable<i4>
  %c = seq.compreg %7, %clk initial %8 : i4
  %16 = hw.constant 0 : i1
  %14 = comb.icmp eq %rst, %16 : i1
  %19 = hw.constant 2 : i4
  %17 = comb.icmp eq %a, %19 : i4
  %13 = comb.and %14, %17 : i1
  %22 = hw.constant 3 : i4
  %20 = comb.icmp eq %b, %22 : i4
  %12 = comb.and %13, %20 : i1
  %26 = hw.constant 5 : i4
  %24 = comb.icmp eq %c, %26 : i4
  %23 = ltl.delay %24, 1, 0 : i1
  %27 = seq.from_clock %clk
  %28 = ltl.implication %12, %23 : i1, !ltl.sequence
  %29 = ltl.clock %28, posedge %27 : !ltl.property
  verif.assert %29 : !ltl.property
  hw.output %c : i4
}

