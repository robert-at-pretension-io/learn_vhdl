hw.module @TestSeq(in %req: i1, in %ack: i1, in %data_valid: i1, in %clk: !seq.clock) {
  // --- Body ---
  %11 = ltl.concat %data_valid, %ack : i1, i1
  %12 = seq.from_clock %clk
  %13 = ltl.clock %6, posedge %12 : !ltl.property
  %18 = ltl.repeat %data_valid, 3 : i1
  %19 = ltl.clock %14, posedge %12 : !ltl.property
  %24 = ltl.repeat %data_valid, 1, 3 : i1
  %25 = ltl.clock %20, posedge %12 : !ltl.property
  %31 = ltl.concat %ack, %data_valid : i1, i1
  %32 = ltl.clock %26, posedge %12 : !ltl.property
  %35 = ltl.delay %ack, 2, 0 : i1
  %37 = ltl.clock %33, posedge %12 : !ltl.property
  %38 = ltl.delay %ack, 3, 0 : i1
  %40 = ltl.clock %38, posedge %12 : !ltl.property
  %41 = comb.and %0, %3 : i1
  verif.assert %41 : i1
  verif.assert %13 : !ltl.property
  verif.assert %19 : !ltl.property
  verif.assert %25 : !ltl.property
  verif.assert %32 : !ltl.property
  verif.assert %37 : !ltl.property
  verif.assert %40 : !ltl.property
  hw.output
}

