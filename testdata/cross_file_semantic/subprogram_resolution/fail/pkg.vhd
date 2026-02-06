library ieee;
use ieee.std_logic_1164.all;

package pkg is
  function f(x : integer) return integer;
  function f(x : natural) return integer;
end package;

package body pkg is
  function f(x : integer) return integer is begin return x; end;
  function f(x : natural) return integer is begin return integer(x); end;
end package body;
