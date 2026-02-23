hw.module @alu(in %opcode: i2, in %a: i8, in %b: i8, out res: i8) {
  // --- Body ---
  %1 = comb.xor %a, %b : i8
  %4 = hw.constant 2 : i2
  %5 = comb.icmp eq %opcode, %4 : i2
  %6 = comb.and %a, %b : i8
  %9 = comb.mux %5, %6, %1 : i8
  %10 = hw.constant 1 : i2
  %11 = comb.icmp eq %opcode, %10 : i2
  %12 = comb.sub %a, %b : i8
  %15 = comb.mux %11, %12, %9 : i8
  %16 = hw.constant 0 : i2
  %17 = comb.icmp eq %opcode, %16 : i2
  %18 = comb.add %a, %b : i8
  %res = comb.mux %17, %18, %15 : i8
  hw.output %res : i8
}

