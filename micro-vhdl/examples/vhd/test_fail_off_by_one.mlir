hw.module @test_fail_off_by_one(in %clk: !seq.clock, in %rst: i1, in %start: i1, out done: i1) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %rst, %2 : i1
  %3 = hw.constant 0 : i1
  %7 = hw.constant 1 : i1
  %5 = comb.icmp eq %start, %7 : i1
  %10 = hw.constant 1 : i1
  %8 = comb.icmp eq %delay1, %10 : i1
  %4 = comb.and %5, %8 : i1
  %13 = comb.mux %4, %delay1, %delay2 : i1
  %14 = comb.mux %0, %3, %13 : i1
  %15 = seq.initial () {
    %16 = hw.constant 0 : i1
    seq.yield %16 : i1
  } : () -> !seq.immutable<i1>
  %done = seq.compreg %14, %clk initial %15 : i1
  %19 = hw.constant 1 : i1
  %17 = comb.icmp eq %rst, %19 : i1
  %20 = hw.constant 0 : i1
  %25 = hw.constant 1 : i1
  %23 = comb.icmp eq %start, %25 : i1
  %28 = hw.constant 1 : i1
  %26 = comb.icmp eq %delay1, %28 : i1
  %22 = comb.and %23, %26 : i1
  %29 = comb.mux %17, %20, %start : i1
  %30 = seq.initial () {
    %31 = hw.constant 0 : i1
    seq.yield %31 : i1
  } : () -> !seq.immutable<i1>
  %delay1 = seq.compreg %29, %clk initial %30 : i1
  %34 = hw.constant 1 : i1
  %32 = comb.icmp eq %rst, %34 : i1
  %35 = hw.constant 0 : i1
  %40 = hw.constant 1 : i1
  %38 = comb.icmp eq %start, %40 : i1
  %43 = hw.constant 1 : i1
  %41 = comb.icmp eq %delay1, %43 : i1
  %37 = comb.and %38, %41 : i1
  %44 = comb.mux %32, %35, %delay1 : i1
  %45 = seq.initial () {
    %46 = hw.constant 0 : i1
    seq.yield %46 : i1
  } : () -> !seq.immutable<i1>
  %delay2 = seq.compreg %44, %clk initial %45 : i1
  %50 = hw.constant 1 : i1
  %48 = comb.icmp eq %start, %50 : i1
  %54 = hw.constant 1 : i1
  %52 = comb.icmp eq %done, %54 : i1
  %51 = ltl.delay %52, 3, 0 : i1
  %55 = seq.from_clock %clk
  %56 = ltl.implication %48, %51 : i1, !ltl.sequence
  %57 = ltl.clock %56, posedge %55 : !ltl.property
  verif.assert %57 : !ltl.property
  hw.output %done : i1
}

