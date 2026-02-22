hw.module @test(in %clk: !seq.clock, in %a: i32, out z: i32) {
  %0 = hw.constant 1 : i32
  %1 = comb.add %a, %0 : i32
  %2 = seq.compreg %1, %clk : i32
  
  // Property to prove
  %c10 = hw.constant 10 : i32
  %ok = comb.icmp ne %2, %c10 : i32
  verif.assert %ok : i1
  
  hw.output %2 : i32
}