hw.module @next_delay_fail(in %clk: !seq.clock, in %req: i1, out ack: i1) {
  // --- Body ---
  %1 = seq.initial () {
    %2 = hw.constant 0 : i1
    seq.yield %2 : i1
  } : () -> !seq.immutable<i1>
  %d1 = seq.compreg %req, %clk initial %1 : i1
  %4 = seq.initial () {
    %5 = hw.constant 0 : i1
    seq.yield %5 : i1
  } : () -> !seq.immutable<i1>
  %d2 = seq.compreg %d1, %clk initial %4 : i1
  %7 = seq.initial () {
    %8 = hw.constant 0 : i1
    seq.yield %8 : i1
  } : () -> !seq.immutable<i1>
  %d3 = seq.compreg %d2, %clk initial %7 : i1
  %10 = seq.initial () {
    %11 = hw.constant 0 : i1
    seq.yield %11 : i1
  } : () -> !seq.immutable<i1>
  %ack = seq.compreg %d3, %clk initial %10 : i1
  %14 = ltl.delay %ack, 2, 0 : i1
  %16 = seq.from_clock %clk
  %17 = hw.constant -1 : i1
  %18 = ltl.concat %req, %17 : i1, i1
  %19 = ltl.implication %18, %14 : !ltl.sequence, !ltl.sequence
  %20 = ltl.clock %19, posedge %16 : !ltl.property
  %23 = ltl.delay %ack, 1, 1 : i1
  %25 = hw.constant -1 : i1
  %26 = ltl.concat %req, %25 : i1, i1
  %27 = ltl.implication %26, %23 : !ltl.sequence, !ltl.sequence
  %28 = ltl.clock %27, posedge %16 : !ltl.property
  verif.assert %20 : !ltl.property
  verif.assert %28 : !ltl.property
  hw.output %ack : i1
}

