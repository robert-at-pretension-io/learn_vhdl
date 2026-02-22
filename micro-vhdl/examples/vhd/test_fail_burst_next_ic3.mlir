hw.module @burst_next(in %req: i1, in %clk: !seq.clock, out valid_out: i1, out __verif_bad: i1) {
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
  %active = seq.compreg %13, %clk : i1
  %16 = hw.constant 0 : i1
  %14 = comb.icmp eq %active, %16 : i1
  %19 = hw.constant 1 : i1
  %17 = comb.icmp eq %req, %19 : i1
  %20 = hw.constant 2 : i2
  %21 = comb.mux %17, %20, %cnt : i2
  %24 = hw.constant 0 : i2
  %22 = comb.icmp eq %cnt, %24 : i2
  %27 = hw.constant 1 : i2
  %25 = comb.sub %cnt, %27 : i2
  %28 = comb.mux %22, %cnt, %25 : i2
  %29 = comb.mux %14, %21, %28 : i2
  %cnt = seq.compreg %29, %clk : i2
  %32 = hw.constant 0 : i1
  %30 = comb.icmp eq %active, %32 : i1
  %35 = hw.constant 1 : i1
  %33 = comb.icmp eq %req, %35 : i1
  %36 = hw.constant 1 : i1
  %37 = hw.constant 0 : i1
  %38 = comb.mux %33, %36, %37 : i1
  %41 = hw.constant 0 : i2
  %39 = comb.icmp eq %cnt, %41 : i2
  %42 = hw.constant 0 : i1
  %43 = hw.constant 1 : i1
  %44 = comb.mux %39, %42, %43 : i1
  %45 = comb.mux %30, %38, %44 : i1
  %valid_out = seq.compreg %45, %clk : i1
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

