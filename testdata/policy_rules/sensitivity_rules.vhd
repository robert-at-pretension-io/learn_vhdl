library ieee;
use ieee.std_logic_1164.all;

entity sensitivity_rules is
  port (
    a : in std_logic;
    b : in std_logic;
    c : in std_logic
  );
end sensitivity_rules;

architecture rtl of sensitivity_rules is
  signal s_out1 : std_logic;
  signal s_out2 : std_logic;
begin
  p_incomplete: process(a)
  begin
    s_out1 <= a and b;
  end process;

  p_super: process(a, b, c)
  begin
    s_out2 <= a or b;
  end process;
end rtl;
