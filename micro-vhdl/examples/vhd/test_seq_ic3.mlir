hw.module @TestSeq(in %req: i1, in %ack: i1, in %data_valid: i1, in %clk: !seq.clock, out __verif_bad: i1) {
  // --- Body ---
  %3 = hw.constant -1 : i1
  %4 = comb.xor %req, %3 : i1
  %0 = comb.or %4, %ack : i1
  %8 = seq.compreg %req, %clk : i1
  %9 = hw.constant -1 : i1
  %10 = comb.xor %8, %9 : i1
  %5 = comb.or %10, %data_valid : i1
  %16 = ltl.concat %data_valid, %ack : i1, i1
  %17 = seq.from_clock %clk
  %18 = hw.constant -1 : i1
  %19 = ltl.concat %req, %18 : i1, i1
  %20 = ltl.implication %19, %16 : !ltl.sequence, !ltl.sequence
  %21 = ltl.clock %20, posedge %17 : !ltl.property
  %26 = ltl.repeat %data_valid, 3 : i1
  %27 = hw.constant -1 : i1
  %28 = ltl.concat %req, %27 : i1, i1
  %29 = ltl.implication %28, %26 : !ltl.sequence, !ltl.sequence
  %30 = ltl.clock %29, posedge %17 : !ltl.property
  %35 = ltl.repeat %data_valid, 1, 3 : i1
  %36 = hw.constant -1 : i1
  %37 = ltl.concat %req, %36 : i1, i1
  %38 = ltl.implication %37, %35 : !ltl.sequence, !ltl.sequence
  %39 = ltl.clock %38, posedge %17 : !ltl.property
  %45 = ltl.concat %ack, %data_valid : i1, i1
  %46 = ltl.implication %req, %45 : i1, !ltl.sequence
  %47 = ltl.clock %46, posedge %17 : !ltl.property
  %50 = ltl.delay %ack, 2, 0 : i1
  %52 = ltl.implication %req, %50 : i1, !ltl.sequence
  %53 = ltl.clock %52, posedge %17 : !ltl.property
  %54 = ltl.delay %ack, 3, 0 : i1
  %56 = ltl.clock %54, posedge %17 : !ltl.property
  %57 = comb.and %0, %5 : i1
  %58 = hw.constant -1 : i1
  %59 = comb.xor %57, %58 : i1
  // temporal assertion skipped in IC3 path (type !ltl.property): %21
  // temporal assertion skipped in IC3 path (type !ltl.property): %30
  // temporal assertion skipped in IC3 path (type !ltl.property): %39
  // temporal assertion skipped in IC3 path (type !ltl.property): %47
  // temporal assertion skipped in IC3 path (type !ltl.property): %53
  // temporal assertion skipped in IC3 path (type !ltl.property): %56
  hw.output %59 : i1
}

