hw.module @my_module(in %clk: !seq.clock, in %idx: i32, in %arr_in: !hw.array<8xi1>, in %rec_in: !hw.struct<valid: i1, data: i8>, out out_sig: i1) {
  // --- Body ---
  %0 = hw.array_get %arr_in[%idx] : !hw.array<8xi1>, i32
  %3 = hw.struct_extract %rec_in["valid"] : !hw.struct<valid: i1, data: i8>
  %out_sig = comb.and %0, %3 : i1
  hw.output %out_sig : i1
}

