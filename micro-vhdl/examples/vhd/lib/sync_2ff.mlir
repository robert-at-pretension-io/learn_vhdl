hw.module private @sync_2ff(in %clk_dst: !seq.clock, in %d_in: i1, out d_out: i1) {
  // --- Body ---
  %1 = seq.initial () {
    %2 = hw.constant 0 : i1
    seq.yield %2 : i1
  } : () -> !seq.immutable<i1>
  %stage1 = seq.compreg %d_in, %clk_dst initial %1 : i1
  %4 = seq.initial () {
    %5 = hw.constant 0 : i1
    seq.yield %5 : i1
  } : () -> !seq.immutable<i1>
  %d_out = seq.compreg %stage1, %clk_dst initial %4 : i1
  hw.output %d_out : i1
}

hw.module @test_cdc_ok(in %clk_slow: !seq.clock, in %clk_fast: !seq.clock, in %data_in: i1, out result: i1) {
  // --- Body ---
  %7 = seq.initial () {
    %8 = hw.constant 0 : i1
    seq.yield %8 : i1
  } : () -> !seq.immutable<i1>
  %data_reg = seq.compreg %data_in, %clk_slow initial %7 : i1
  %data_sync = hw.instance "u_sync" @sync_2ff(clk_dst: %clk_fast: !seq.clock, d_in: %data_reg: i1) -> (d_out: i1)
  %12 = seq.initial () {
    %13 = hw.constant 0 : i1
    seq.yield %13 : i1
  } : () -> !seq.immutable<i1>
  %result = seq.compreg %data_sync, %clk_fast initial %12 : i1
  hw.output %result : i1
}

