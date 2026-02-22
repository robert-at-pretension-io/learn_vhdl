hw.module @next_delay_demo(in %clk: !seq.clock, in %req: i1, out ack: i1) {
  // --- Body ---
  %1 = seq.initial () {
    %2 = hw.constant 0 : i1
    seq.yield %2 : i1
  } : () -> !seq.immutable<i1>
  %d2 = seq.compreg %d1, %clk initial %1 : i1
  %4 = seq.initial () {
    %5 = hw.constant 0 : i1
    seq.yield %5 : i1
  } : () -> !seq.immutable<i1>
  %d3 = seq.compreg %d2, %clk initial %4 : i1
  %7 = seq.initial () {
    %8 = hw.constant 0 : i1
    seq.yield %8 : i1
  } : () -> !seq.immutable<i1>
  %ack = seq.compreg %d3, %clk initial %7 : i1
  %10 = seq.initial () {
    %11 = hw.constant 0 : i1
    seq.yield %11 : i1
  } : () -> !seq.immutable<i1>
  %d1 = seq.compreg %req, %clk initial %10 : i1
  %15 = seq.from_clock %clk
  %16 = ltl.delay %ack, 4, 0 : i1
  %17 = ltl.implication %req, %16 : i1, !ltl.sequence
  %18 = ltl.clock %17, posedge %15 : !ltl.property
  %22 = ltl.delay %ack, 2, 3 : i1
  %23 = ltl.implication %req, %22 : i1, !ltl.sequence
  %24 = ltl.clock %23, posedge %15 : !ltl.property
  verif.assert %18 : !ltl.property
  verif.assert %24 : !ltl.property
  hw.output %ack : i1
}

