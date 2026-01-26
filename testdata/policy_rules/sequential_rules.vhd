library ieee;
use ieee.std_logic_1164.all;

entity sequential_rules is
  port (
    clk  : in std_logic;
    rst  : in std_logic;
    data : in std_logic;
    q    : out std_logic
  );
end sequential_rules;

architecture rtl of sequential_rules is
  signal reset_reg : std_logic;
  signal shared_sig : std_logic;
  signal rise_sig : std_logic;
  signal fall_sig : std_logic;
  signal wide0 : std_logic;
  signal wide1 : std_logic;
  signal wide2 : std_logic;
  signal wide3 : std_logic;
  signal wide4 : std_logic;
  signal wide5 : std_logic;
  signal wide6 : std_logic;
  signal wide7 : std_logic;
  signal wide8 : std_logic;
  signal wide9 : std_logic;
  signal wide10 : std_logic;
  signal wide11 : std_logic;
  signal wide12 : std_logic;
  signal wide13 : std_logic;
  signal wide14 : std_logic;
  signal wide15 : std_logic;
begin
  seq_proc: process(data)
  begin
    if rising_edge(clk) then
      shared_sig <= data;
    end if;
  end process;

  comb_proc: process(data, shared_sig)
  begin
    shared_sig <= data;
  end process;

  p_reset_missing: process(clk)
  begin
    if rst = '1' then
      reset_reg <= '0';
    elsif rising_edge(clk) then
      reset_reg <= data;
    end if;
  end process;

  p_rise: process(clk)
  begin
    if rising_edge(clk) then
      rise_sig <= data;
    end if;
  end process;

  p_fall: process(clk)
  begin
    if falling_edge(clk) then
      fall_sig <= data;
    end if;
  end process;

  p_wide: process(clk)
  begin
    if rising_edge(clk) then
      wide0 <= data;
      wide1 <= data;
      wide2 <= data;
      wide3 <= data;
      wide4 <= data;
      wide5 <= data;
      wide6 <= data;
      wide7 <= data;
      wide8 <= data;
      wide9 <= data;
      wide10 <= data;
      wide11 <= data;
      wide12 <= data;
      wide13 <= data;
      wide14 <= data;
      wide15 <= data;
    end if;
  end process;

  q <= shared_sig;
end rtl;
