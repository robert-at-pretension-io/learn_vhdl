hw.module @TestSeq(in %req: i1, in %ack: i1, in %data_valid: i1, in %clk: !seq.clock) {
  // --- Body ---
  %11 = ltl.concat %data_valid, %ack : i1, i1
  %16 = ltl.repeat %data_valid, 3 : i1
  %21 = ltl.repeat %data_valid, 1, 3 : i1
  %27 = ltl.concat %ack, %data_valid : i1, i1
  %30 = ltl.delay %ack, 2, 0 : i1
  %32 = ltl.delay %ack, 3, 0 : i1
  %34 = comb.and %0, %3 : i1
  verif.assert %34 : i1
  %35 = seq.from_clock %clk
  %36 = ltl.clock %6, posedge %35 : !ltl.property
  verif.assert %36 : !ltl.property
  %37 = ltl.clock %12, posedge %35 : !ltl.property
  verif.assert %37 : !ltl.property
  %38 = ltl.clock %17, posedge %35 : !ltl.property
  verif.assert %38 : !ltl.property
  %39 = ltl.clock %22, posedge %35 : !ltl.property
  verif.assert %39 : !ltl.property
  %40 = ltl.clock %28, posedge %35 : !ltl.property
  verif.assert %40 : !ltl.property
  %41 = ltl.clock %32, posedge %35 : !ltl.property
  verif.assert %41 : !ltl.property
  hw.output
}

