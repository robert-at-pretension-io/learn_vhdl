library ieee;
use ieee.std_logic_1164.all;

package math_pkg is
  function f(a : integer; c : integer) return integer;
end;

package body math_pkg is
  function f(a : integer; c : integer) return integer is
  begin
    return a + c;
  end function;
end;

entity call_nested is
  port(
    a : in integer;
    b : in integer;
    y : out integer
  );
end;

architecture rtl of call_nested is
  function g(x : integer) return integer is
  begin
    return x + 1;
  end function;

  function h(x : integer) return integer is
  begin
    return x * 2;
  end function;
begin
  p_nested : process
  begin
    y <= math_pkg.f(a => g(b), c => h(1));
    wait;
  end process;
end;
