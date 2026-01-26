library ieee;
use ieee.std_logic_1164.all;

entity rdc_rules is
  port (
    clk_a : in std_logic;
    clk_b : in std_logic;
    a     : in std_logic;
    b     : in std_logic
  );
end rdc_rules;

architecture rtl of rdc_rules is
  signal rst_n : std_logic;
  signal rst_sync : std_logic;
  signal q1    : std_logic;
  signal q2    : std_logic;
  signal q3    : std_logic;
begin
  rst_n <= a and b;

  proc_a: process(clk_a, rst_n)
  begin
    if rst_n = '0' then
      q1 <= '0';
    elsif rising_edge(clk_a) then
      q1 <= a;
    end if;
  end process;

  proc_b: process(clk_b, rst_n)
  begin
    if rst_n = '0' then
      q2 <= '0';
    elsif rising_edge(clk_b) then
      q2 <= b;
    end if;
  end process;

  proc_a_no_reset: process(clk_a)
  begin
    if rising_edge(clk_a) then
      q3 <= a;
    end if;
  end process;

  proc_sync: process(clk_a)
  begin
    if rising_edge(clk_a) then
      rst_sync <= rst_n;
    end if;
  end process;
end rtl;
