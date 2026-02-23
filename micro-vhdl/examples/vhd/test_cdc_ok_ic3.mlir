hw.module @test_cdc_ok(in %clk_slow: !seq.clock, in %clk_fast: !seq.clock, in %data_in: i1, out result: i1) {
  // --- Body ---
  %data_reg = seq.compreg %data_in, %clk_slow : i1
  %data_sync = hw.instance "u_sync" @sync_2ff(clk_dst: %clk_fast: !seq.clock, d_in: %data_reg: i1) -> (d_out: i1)
  %result = seq.compreg %data_sync, %clk_fast : i1
  hw.output %result : i1
}

hw.module private @sync_2ff(in %clk_dst: !seq.clock, in %d_in: i1, out d_out: i1) {
  // --- Body ---
  %stage1 = seq.compreg %d_in, %clk_dst : i1
  %d_out = seq.compreg %stage1, %clk_dst : i1
  hw.output %d_out : i1
}

