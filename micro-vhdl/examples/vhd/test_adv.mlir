hw.module @my_module(in %clk: !seq.clock, in %idx: i32, in %arr_in: i1, in %rec_in: i1, out out_sig: i1) {
  // --- Body ---
  %0 = hw.array_get %arr_in[%idx] : i1, i32
  %3 = hw.struct_extract %rec_in["valid"] : i1
  %out_sig = comb.and %0, %3 : i1
  hw.output %out_sig : i1
}

