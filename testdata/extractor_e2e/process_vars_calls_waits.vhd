library ieee;
use ieee.std_logic_1164.all;

entity call_top is
  port(
    clk : in std_logic;
    a   : in std_logic;
    y   : out std_logic
  );
end;

architecture rtl of call_top is
  procedure poke(signal s : in std_logic) is
  begin
    null;
  end procedure;

  function f(x : std_logic) return std_logic is
  begin
    return x;
  end function;

begin
  p_call : process
    variable v : integer;
  begin
    wait until rising_edge(clk);
    v := 1;
    poke(a);
    y <= f(a);
  end process;
end;
