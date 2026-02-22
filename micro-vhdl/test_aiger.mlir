hw.module @test(in %clk: !seq.clock, out bad: i1, out justice: i1) {
  %0 = hw.constant 1 : i1
  %1 = hw.constant 0 : i1
  hw.output %0, %1 : i1, i1
}
