library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

package p is
  function f(x : integer) return integer;
end;

package body p is
  function f(x : integer) return integer is
  begin
    return x + 1;
  end function;
end;

entity call_arrays is
  port(
    clk : in std_logic;
    a   : in integer;
    y   : out integer
  );
end;

architecture rtl of call_arrays is
  type int_arr_t is array (0 to 3) of integer;
  signal arr : int_arr_t := (others => 0);
begin
  p1 : process(clk)
  begin
    if rising_edge(clk) then
      arr(0) <= a;
      y <= p.f(arr(1));
    end if;
  end process;
end;
