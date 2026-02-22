hw.module @burst_next(in %req: i1, in %clk: !seq.clock, out valid_out: i1, out __verif_bad: i1) {
  // --- Body ---
  %2 = hw.constant 0 : i1
  %0 = comb.icmp eq %active, %2 : i1
  %5 = hw.constant 1 : i1
  %3 = comb.icmp eq %req, %5 : i1
  %6 = hw.constant 1 : i1
  %7 = hw.constant 0 : i1
  %8 = comb.mux %3, %6, %7 : i1
  %11 = hw.constant 0 : i2
  %9 = comb.icmp eq %cnt, %11 : i2
  %12 = hw.constant 0 : i1
  %13 = hw.constant 1 : i1
  %14 = comb.mux %9, %12, %13 : i1
  %15 = comb.mux %0, %8, %14 : i1
  %valid_out = seq.compreg %15, %clk : i1
  %18 = hw.constant 0 : i1
  %16 = comb.icmp eq %active, %18 : i1
  %21 = hw.constant 1 : i1
  %19 = comb.icmp eq %req, %21 : i1
  %22 = hw.constant 1 : i1
  %23 = comb.mux %19, %22, %active : i1
  %26 = hw.constant 0 : i2
  %24 = comb.icmp eq %cnt, %26 : i2
  %27 = hw.constant 0 : i1
  %28 = comb.mux %24, %27, %active : i1
  %29 = comb.mux %16, %23, %28 : i1
  %active = seq.compreg %29, %clk : i1
  %32 = hw.constant 0 : i1
  %30 = comb.icmp eq %active, %32 : i1
  %35 = hw.constant 1 : i1
  %33 = comb.icmp eq %req, %35 : i1
  %36 = hw.constant 2 : i2
  %37 = comb.mux %33, %36, %cnt : i2
  %40 = hw.constant 0 : i2
  %38 = comb.icmp eq %cnt, %40 : i2
  %43 = hw.constant 1 : i2
  %41 = comb.sub %cnt, %43 : i2
  %44 = comb.mux %38, %cnt, %41 : i2
  %45 = comb.mux %30, %37, %44 : i2
  %cnt = seq.compreg %45, %clk : i2
  %49 = hw.constant -1 : i1
  %50 = comb.xor %req, %49 : i1
  %46 = comb.or %50, %valid_out : i1
  %53 = ltl.delay %valid_out, 1, 0 : i1
  %55 = seq.from_clock %clk
  %56 = ltl.implication %req, %53 : i1, !ltl.sequence
  %57 = ltl.clock %56, posedge %55 : !ltl.property
  %60 = ltl.delay %valid_out, 2, 0 : i1
  %62 = ltl.implication %req, %60 : i1, !ltl.sequence
  %63 = ltl.clock %62, posedge %55 : !ltl.property
  %66 = ltl.delay %valid_out, 3, 0 : i1
  %68 = ltl.implication %req, %66 : i1, !ltl.sequence
  %69 = ltl.clock %68, posedge %55 : !ltl.property
  %70 = hw.constant -1 : i1
  %71 = comb.xor %46, %70 : i1
  // temporal assertion skipped in IC3 path (type !ltl.property): %57
  // temporal assertion skipped in IC3 path (type !ltl.property): %63
  // temporal assertion skipped in IC3 path (type !ltl.property): %69
  hw.output %valid_out, %71 : i1, i1
}

