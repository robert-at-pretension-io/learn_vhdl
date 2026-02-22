hw.module @test(in %clk: !seq.clock, in %rst: i1, in %a: i32, out z: i32) {
  %0 = hw.constant 1 : i32
  %1 = comb.add %a, %0 : i32
  %2 = seq.compreg %1, %clk : i32
  hw.output %2 : i32
}
