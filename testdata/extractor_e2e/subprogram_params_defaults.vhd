library ieee;
use ieee.std_logic_1164.all;

package params_pkg is
  function f1(constant a : in integer := 3; b : integer) return integer;
  procedure p1(signal clk : in std_logic; variable v : inout integer; constant c : integer := 7);
end package;
