hw.module @sequence_burst(in %req: i1, in %clk: !seq.clock, out valid_out: i1) {
  // --- Body ---
  %2 = hw.constant 0 : i1
  %0 = comb.icmp eq %active, %2 : i1
  %5 = hw.constant 1 : i1
  %3 = comb.icmp eq %req, %5 : i1
  %6 = hw.constant 2 : i2
  %7 = comb.mux %3, %6, %cnt : i2
  %10 = hw.constant 0 : i2
  %8 = comb.icmp eq %cnt, %10 : i2
  %13 = hw.constant 1 : i2
  %11 = comb.sub %cnt, %13 : i2
  %14 = comb.mux %8, %cnt, %11 : i2
  %15 = comb.mux %0, %7, %14 : i2
  %16 = seq.initial () {
    %17 = hw.constant 0 : i2
    seq.yield %17 : i2
  } : () -> !seq.immutable<i2>
  %cnt = seq.compreg %15, %clk initial %16 : i2
  %20 = hw.constant 0 : i1
  %18 = comb.icmp eq %active, %20 : i1
  %23 = hw.constant 1 : i1
  %21 = comb.icmp eq %req, %23 : i1
  %24 = hw.constant 1 : i1
  %25 = hw.constant 0 : i1
  %26 = comb.mux %21, %24, %25 : i1
  %29 = hw.constant 0 : i2
  %27 = comb.icmp eq %cnt, %29 : i2
  %30 = hw.constant 0 : i1
  %31 = hw.constant 1 : i1
  %32 = comb.mux %27, %30, %31 : i1
  %33 = comb.mux %18, %26, %32 : i1
  %34 = seq.initial () {
    %35 = hw.constant 0 : i1
    seq.yield %35 : i1
  } : () -> !seq.immutable<i1>
  %valid_out = seq.compreg %33, %clk initial %34 : i1
  %38 = hw.constant 0 : i1
  %36 = comb.icmp eq %active, %38 : i1
  %41 = hw.constant 1 : i1
  %39 = comb.icmp eq %req, %41 : i1
  %42 = hw.constant 1 : i1
  %43 = comb.mux %39, %42, %active : i1
  %46 = hw.constant 0 : i2
  %44 = comb.icmp eq %cnt, %46 : i2
  %47 = hw.constant 0 : i1
  %48 = comb.mux %44, %47, %active : i1
  %49 = comb.mux %36, %43, %48 : i1
  %50 = seq.initial () {
    %51 = hw.constant 0 : i1
    seq.yield %51 : i1
  } : () -> !seq.immutable<i1>
  %active = seq.compreg %49, %clk initial %50 : i1
  %56 = ltl.repeat %valid_out, 4 : i1
  %57 = seq.from_clock %clk
  %58 = ltl.implication %req, %56 : i1, !ltl.sequence
  %59 = ltl.clock %58, posedge %57 : !ltl.property
  verif.assert %59 : !ltl.property
  hw.output %valid_out : i1
}

