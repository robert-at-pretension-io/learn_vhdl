hw.module @timer(in %clk: !seq.clock, in %reset: i1, in %en: i1, in %limit: i8, out done: i1) {
  // --- Body ---
  %0 = hw.constant 1 : i1
  %1 = comb.icmp eq %count, %limit : i8
  %4 = hw.constant 0 : i1
  %is_done = comb.mux %1, %0, %4 : i1
  %7 = hw.constant 1 : i8
  %5 = comb.add %count, %7 : i8
  %10 = hw.constant 0 : i1
  %8 = comb.icmp eq %is_done, %10 : i1
  %11 = hw.constant 0 : i8
  %next_count = comb.mux %8, %5, %11 : i8
  %14 = hw.constant 1 : i1
  %12 = comb.icmp eq %reset, %14 : i1
  %15 = hw.constant 0 : i8
  %19 = hw.constant 1 : i1
  %17 = comb.icmp eq %en, %19 : i1
  %21 = comb.mux %17, %next_count, %count : i8
  %22 = comb.mux %12, %15, %21 : i8
  %count = seq.compreg %22, %clk : i8
  hw.output %is_done : i1
}

