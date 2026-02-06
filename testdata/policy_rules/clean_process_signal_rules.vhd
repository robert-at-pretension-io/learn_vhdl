-- Negative fixture for Batch 1: process & signal rules
-- Clean code that should NOT trigger any of the new rules.

library ieee;
use ieee.std_logic_1164.all;

entity clean_process_signal_ent is
  port (
    clk   : in  std_logic;
    rst_n : in  std_logic;
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity clean_process_signal_ent;

architecture rtl of clean_process_signal_ent is
  signal data_reg : std_logic;
begin

  -- Good sequential process: has clock edge, no wait
  seq_proc : process(clk, rst_n)
  begin
    if rst_n = '0' then
      data_reg <= '0';
    elsif rising_edge(clk) then
      data_reg <= din;
    end if;
  end process seq_proc;

  -- Good combinational process: has statements, signals in sensitivity list
  comb_proc : process(data_reg)
  begin
    dout <= data_reg;
  end process comb_proc;

end architecture rtl;
