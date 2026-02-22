hw.module @TestSeq(in %req: i1, in %ack: i1, in %data_valid: i1, in %clk: !seq.clock) {
  // --- Body ---
  %3 = hw.constant -1 : i1
  %4 = comb.xor %req, %3 : i1
  %0 = comb.or %4, %ack : i1
  %8 = seq.initial () {
    %9 = hw.constant 0 : i1
    seq.yield %9 : i1
  } : () -> !seq.immutable<i1>
  %10 = seq.compreg %req, %clk initial %8 : i1
  %11 = hw.constant -1 : i1
  %12 = comb.xor %10, %11 : i1
  %5 = comb.or %12, %data_valid : i1
  %18 = ltl.concat %data_valid, %ack : i1, i1
  %19 = seq.from_clock %clk
  %20 = hw.constant -1 : i1
  %21 = ltl.concat %req, %20 : i1, i1
  %22 = ltl.implication %21, %18 : !ltl.sequence, !ltl.sequence
  %23 = ltl.clock %22, posedge %19 : !ltl.property
  %28 = ltl.repeat %data_valid, 3 : i1
  %29 = hw.constant -1 : i1
  %30 = ltl.concat %req, %29 : i1, i1
  %31 = ltl.implication %30, %28 : !ltl.sequence, !ltl.sequence
  %32 = ltl.clock %31, posedge %19 : !ltl.property
  %37 = ltl.repeat %data_valid, 1, 3 : i1
  %38 = hw.constant -1 : i1
  %39 = ltl.concat %req, %38 : i1, i1
  %40 = ltl.implication %39, %37 : !ltl.sequence, !ltl.sequence
  %41 = ltl.clock %40, posedge %19 : !ltl.property
  %47 = ltl.concat %ack, %data_valid : i1, i1
  %48 = ltl.implication %req, %47 : i1, !ltl.sequence
  %49 = ltl.clock %48, posedge %19 : !ltl.property
  %52 = ltl.delay %ack, 2, 0 : i1
  %54 = ltl.implication %req, %52 : i1, !ltl.sequence
  %55 = ltl.clock %54, posedge %19 : !ltl.property
  %56 = ltl.delay %ack, 3, 0 : i1
  %58 = ltl.clock %56, posedge %19 : !ltl.property
  %59 = comb.and %0, %5 : i1
  verif.assert %59 : i1
  verif.assert %23 : !ltl.property
  verif.assert %32 : !ltl.property
  verif.assert %41 : !ltl.property
  verif.assert %49 : !ltl.property
  verif.assert %55 : !ltl.property
  verif.assert %58 : !ltl.property
  hw.output
}

