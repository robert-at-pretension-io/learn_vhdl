hw.module @test_fail_off_by_one(in %clk: !seq.clock, in %rst: i1, in %start: i1, out done: i1, out __verif_bad: i1) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %rst, %2 : i1
  %3 = hw.constant 0 : i1
  %8 = hw.constant 1 : i1
  %6 = comb.icmp eq %start, %8 : i1
  %11 = hw.constant 1 : i1
  %9 = comb.icmp eq %delay1, %11 : i1
  %5 = comb.and %6, %9 : i1
  %12 = comb.mux %0, %3, %start : i1
  %delay1 = seq.compreg %12, %clk : i1
  %15 = hw.constant 1 : i1
  %13 = comb.icmp eq %rst, %15 : i1
  %16 = hw.constant 0 : i1
  %21 = hw.constant 1 : i1
  %19 = comb.icmp eq %start, %21 : i1
  %24 = hw.constant 1 : i1
  %22 = comb.icmp eq %delay1, %24 : i1
  %18 = comb.and %19, %22 : i1
  %25 = comb.mux %13, %16, %delay1 : i1
  %delay2 = seq.compreg %25, %clk : i1
  %28 = hw.constant 1 : i1
  %26 = comb.icmp eq %rst, %28 : i1
  %29 = hw.constant 0 : i1
  %33 = hw.constant 1 : i1
  %31 = comb.icmp eq %start, %33 : i1
  %36 = hw.constant 1 : i1
  %34 = comb.icmp eq %delay1, %36 : i1
  %30 = comb.and %31, %34 : i1
  %39 = comb.mux %30, %delay1, %delay2 : i1
  %40 = comb.mux %26, %29, %39 : i1
  %done = seq.compreg %40, %clk : i1
  %44 = hw.constant 1 : i1
  %42 = comb.icmp eq %start, %44 : i1
  %48 = hw.constant 1 : i1
  %46 = comb.icmp eq %done, %48 : i1
  %45 = ltl.delay %46, 3, 0 : i1
  %49 = seq.from_clock %clk
  %50 = ltl.implication %42, %45 : i1, !ltl.sequence
  %51 = ltl.clock %50, posedge %49 : !ltl.property
  // temporal assertion skipped in IC3 path (type !ltl.property): %51
  %52 = hw.constant 0 : i1
  hw.output %done, %52 : i1, i1
}

