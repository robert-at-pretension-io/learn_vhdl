-- Positive fixture for Batch 1: process & signal rules
-- Each construct triggers one specific rule.

library ieee;
use ieee.std_logic_1164.all;

entity process_signal_ent is
  port (
    clk   : in  std_logic;
    rst_n : in  std_logic;
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity process_signal_ent;

architecture rtl of process_signal_ent is
  signal data_reg : std_logic;
  shared variable sv_count : integer := 0;  -- shared_variable_usage
begin

  -- wait_in_clocked_process: sequential with rising_edge AND wait
  wait_clk_proc : process(clk)
  begin
    if rising_edge(clk) then
      data_reg <= din;
    end if;
    wait for 10 ns;
  end process wait_clk_proc;

  -- process_with_no_statements: empty process body
  empty_proc : process(clk)
  begin
    -- no assignments, no calls, no waits
  end process empty_proc;

  -- variable_in_sensitivity_list: variable in sensitivity list
  var_sens_proc : process(v_tmp)
    variable v_tmp : std_logic;
  begin
    v_tmp := din;
    dout <= v_tmp;
  end process var_sens_proc;

  -- constant_if_condition: comparing signal to itself
  self_cmp_proc : process(data_reg)
  begin
    if data_reg = data_reg then
      dout <= '1';
    end if;
  end process self_cmp_proc;

end architecture rtl;

-- duplicate_architecture_name: second architecture with same name for same entity
architecture rtl of process_signal_ent is
begin
  dout <= din;
end architecture rtl;
