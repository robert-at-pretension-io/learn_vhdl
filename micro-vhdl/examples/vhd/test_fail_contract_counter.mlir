hw.module @bounded_counter(in %inc: i1, in %clk: !seq.clock, out val: i4) {
  // --- Body ---
  %2 = hw.constant 1 : i1
  %0 = comb.icmp eq %inc, %2 : i1
  %5 = hw.constant 1 : i4
  %3 = comb.add %count, %5 : i4
  %6 = comb.mux %0, %3, %count : i4
  %7 = seq.initial () {
    %8 = hw.constant 0 : i4
    seq.yield %8 : i4
  } : () -> !seq.immutable<i4>
  %count = seq.compreg %6, %clk initial %7 : i4
  %9 = verif.contract %count : i4 {
    %12 = hw.constant 15 : i4
    %10 = comb.icmp ne %9, %12 : i4
    verif.ensure %10 : i1
  }
  hw.output %9 : i4
}

