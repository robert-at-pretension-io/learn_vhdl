hw.module private @processor_core<DATA_WIDTH: i32 = 16>(in %clk: !seq.clock, in %reset: i1, in %en: i1, in %opcode: i2, in %operandA: i1, in %operandB: i1, out result: i1, out __verif_bad: i1) {
  // --- Body ---
  %1 = comb.xor %operandA, %operandB : i1
  %4 = hw.constant 2 : i2
  %5 = comb.icmp eq %opcode, %4 : i2
  %6 = comb.and %operandA, %operandB : i1
  %9 = comb.mux %5, %6, %1 : i1
  %10 = hw.constant 1 : i2
  %11 = comb.icmp eq %opcode, %10 : i2
  %12 = comb.sub %operandA, %operandB : i1
  %15 = comb.mux %11, %12, %9 : i1
  %16 = hw.constant 0 : i2
  %17 = comb.icmp eq %opcode, %16 : i2
  %18 = comb.add %operandA, %operandB : i1
  %alu_out = comb.mux %17, %18, %15 : i1
  %22 = hw.constant 1 : i1
  %25 = hw.constant 0 : i1
  %23 = comb.icmp eq %alu_out, %25 : i1
  %26 = hw.constant 0 : i1
  %is_zero = comb.mux %23, %22, %26 : i1
  %29 = hw.constant 1 : i1
  %27 = comb.icmp eq %reset, %29 : i1
  %30 = hw.constant 0 : i1
  %34 = hw.constant 1 : i1
  %32 = comb.icmp eq %en, %34 : i1
  %36 = comb.mux %32, %alu_out, %reg_out : i1
  %37 = comb.mux %27, %30, %36 : i1
  %reg_out = seq.compreg %37, %clk : i1
  %41 = seq.compreg %reset, %clk : i1
  %42 = hw.constant -1 : i1
  %43 = comb.xor %41, %42 : i1
  %38 = comb.or %43, %is_zero : i1
  %46 = seq.compreg %clk, %clk : i1
  %44 = comb.icmp eq %clk, %46 : i1
  %47 = comb.and %38, %44 : i1
  %48 = hw.constant -1 : i1
  %49 = comb.xor %47, %48 : i1
  hw.output %reg_out, %49 : i1, i1
}

hw.module @system_top(in %sys_clk: i1, in %sys_rst: i1, in %sys_en: i1, in %opA_0: i32, in %opB_0: i32, in %opA_1: i32, in %opB_1: i32, out out_0: i32, out out_1: i32) {
  // --- Body ---
  %53 = hw.constant 0 : i2
  %out_0 = hw.instance "core_0" @processor_core<DATA_WIDTH: i32 = 32>(clk: %sys_clk: !seq.clock, reset: %sys_rst: i1, en: %sys_en: i1, opcode: %53: i2, operandA: %opA_0: i1, operandB: %opB_0: i1) -> (result: i1)
  %59 = hw.constant 2 : i2
  %out_1 = hw.instance "core_1" @processor_core<DATA_WIDTH: i32 = 32>(clk: %sys_clk: !seq.clock, reset: %sys_rst: i1, en: %sys_en: i1, opcode: %59: i2, operandA: %opA_1: i1, operandB: %opB_1: i1) -> (result: i1)
  hw.output %out_0, %out_1 : i32, i32
}

