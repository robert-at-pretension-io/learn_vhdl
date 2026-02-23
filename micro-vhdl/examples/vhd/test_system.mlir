hw.module private @processor_core<DATA_WIDTH: i32 = 16>(in %clk: !seq.clock, in %reset: i1, in %en: i1, in %opcode: i2, in %operandA: i1, in %operandB: i1, out result: i1) {
  // --- Body ---
  hw.output %operandA : i1
}

hw.module @system_top(in %sys_clk: i1, in %sys_rst: i1, in %sys_en: i1, in %opA_0: i32, in %opB_0: i32, out out_0: i32) {
  // --- Body ---
  %3 = hw.constant 0 : i2
  %out_0 = hw.instance "core_0" @processor_core<DATA_WIDTH: i32 = 32>(clk: %sys_clk: !seq.clock, reset: %sys_rst: i1, en: %sys_en: i1, opcode: %3: i2, operandA: %opA_0: i1, operandB: %opB_0: i1) -> (result: i1)
  hw.output %out_0 : i32
}

