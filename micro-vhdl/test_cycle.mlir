hw.module @test(%clk: i1) -> (q: i1) {
  %q = seq.compreg %d, %clk : i1
  %d = comb.xor %q, %q : i1
  hw.output %q : i1
}