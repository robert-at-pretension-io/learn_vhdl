hw.module @burst_next(in %req: i1, in %clk: !seq.clock, out valid_out: i1) {
  // --- Body ---
  %2 = hw.constant 0 : i1
  %0 = comb.icmp eq %active, %2 : i1
  %5 = hw.constant 1 : i1
  %3 = comb.icmp eq %req, %5 : i1
  %6 = hw.constant 1 : i1
  %7 = comb.mux %3, %6, %active : i1
  %10 = hw.constant 0 : i2
  %8 = comb.icmp eq %cnt, %10 : i2
  %11 = hw.constant 0 : i1
  %12 = comb.mux %8, %11, %active : i1
  %13 = comb.mux %0, %7, %12 : i1
  %14 = seq.initial () {
    %15 = hw.constant 0 : i1
    seq.yield %15 : i1
  } : () -> !seq.immutable<i1>
  %active = seq.compreg %13, %clk initial %14 : i1
  %18 = hw.constant 0 : i1
  %16 = comb.icmp eq %active, %18 : i1
  %21 = hw.constant 1 : i1
  %19 = comb.icmp eq %req, %21 : i1
  %22 = hw.constant 2 : i2
  %23 = comb.mux %19, %22, %cnt : i2
  %26 = hw.constant 0 : i2
  %24 = comb.icmp eq %cnt, %26 : i2
  %29 = hw.constant 1 : i2
  %27 = comb.sub %cnt, %29 : i2
  %30 = comb.mux %24, %cnt, %27 : i2
  %31 = comb.mux %16, %23, %30 : i2
  %32 = seq.initial () {
    %33 = hw.constant 0 : i2
    seq.yield %33 : i2
  } : () -> !seq.immutable<i2>
  %cnt = seq.compreg %31, %clk initial %32 : i2
  %36 = hw.constant 0 : i1
  %34 = comb.icmp eq %active, %36 : i1
  %39 = hw.constant 1 : i1
  %37 = comb.icmp eq %req, %39 : i1
  %40 = hw.constant 1 : i1
  %41 = hw.constant 0 : i1
  %42 = comb.mux %37, %40, %41 : i1
  %45 = hw.constant 0 : i2
  %43 = comb.icmp eq %cnt, %45 : i2
  %46 = hw.constant 0 : i1
  %47 = hw.constant 1 : i1
  %48 = comb.mux %43, %46, %47 : i1
  %49 = comb.mux %34, %42, %48 : i1
  %50 = seq.initial () {
    %51 = hw.constant 0 : i1
    seq.yield %51 : i1
  } : () -> !seq.immutable<i1>
  %valid_out = seq.compreg %49, %clk initial %50 : i1
  %55 = hw.constant -1 : i1
  %56 = comb.xor %req, %55 : i1
  %52 = comb.or %56, %valid_out : i1
  %59 = ltl.delay %valid_out, 1, 0 : i1
  %61 = seq.from_clock %clk
  %62 = ltl.implication %req, %59 : i1, !ltl.sequence
  %63 = ltl.clock %62, posedge %61 : !ltl.property
  %66 = ltl.delay %valid_out, 2, 0 : i1
  %68 = ltl.implication %req, %66 : i1, !ltl.sequence
  %69 = ltl.clock %68, posedge %61 : !ltl.property
  %72 = ltl.delay %valid_out, 3, 0 : i1
  %74 = ltl.implication %req, %72 : i1, !ltl.sequence
  %75 = ltl.clock %74, posedge %61 : !ltl.property
  verif.assert %52 : i1
  verif.assert %63 : !ltl.property
  verif.assert %69 : !ltl.property
  verif.assert %75 : !ltl.property
  hw.output %valid_out : i1
}

