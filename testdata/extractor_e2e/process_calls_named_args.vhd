library ieee;
use ieee.std_logic_1164.all;

entity call_named is
  port(
    a : in std_logic;
    b : in std_logic;
    y : out std_logic
  );
end;

architecture rtl of call_named is
  function g(x : std_logic; y : std_logic) return std_logic is
  begin
    return x and y;
  end function;
begin
  p_waits : process
  begin
    y <= g(y => b, x => a);
    wait on a, b;
    wait for 10 ns;
  end process;
end;
