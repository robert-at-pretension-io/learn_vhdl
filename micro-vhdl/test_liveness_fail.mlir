hw.module @test(in %clk: !seq.clock, in %req: i1, out assert_fair: i1) {
  // If req happens, we must eventually get ack. But ack is 0. So it should fail!
  %ack = hw.constant 0 : i1
  
  // What is the formulation of "req -> eventually ack"? 
  // Maybe assert_fair is simply the signal that must be 1 infinitely often?
  // Let's just output ack as assert_fair to see what happens.
  hw.output %ack : i1
}