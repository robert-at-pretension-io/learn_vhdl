library ieee;
use ieee.std_logic_1164.all;

entity sensitivity_rules is
  port (
    a : in std_logic;
    b : in std_logic;
    c : in std_logic;
    d : in std_logic;
    e : in std_logic
  );
end sensitivity_rules;

architecture rtl of sensitivity_rules is
  signal s_out1 : std_logic;
  signal s_out2 : std_logic;
  signal s_out3 : std_logic;
  signal s_out4 : std_logic;
  signal s_out5 : std_logic;
begin
  -- Basic: one signal missing from sensitivity
  p_incomplete: process(a)
  begin
    s_out1 <= a and b;
  end process;

  -- Basic: one superfluous signal in sensitivity
  p_super: process(a, b, c)
  begin
    s_out2 <= a or b;
  end process;

  -- Edge: multiple signals missing from sensitivity (nested if/else)
  p_multi_miss: process(a)
  begin
    if a = '1' then
      s_out3 <= b and c;
    else
      s_out3 <= d;
    end if;
  end process;

  -- Edge: signal read only in nested condition, missing from sensitivity
  p_nested_cond: process(a, b)
  begin
    s_out4 <= '0';
    if a = '1' then
      if b = '1' then
        s_out4 <= d;
      else
        s_out4 <= e;
      end if;
    end if;
  end process;

  -- Edge: multiple superfluous signals (4 unused in sensitivity)
  p_multi_super: process(a, b, c, d, e)
  begin
    s_out5 <= a;
  end process;
end rtl;
