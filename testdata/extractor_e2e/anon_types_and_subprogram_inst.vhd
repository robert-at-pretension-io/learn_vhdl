library ieee;
use ieee.std_logic_1164.all;

entity anon_types is
  port (
    a : type is private;
    b : type is <>;
    c : linkage std_logic
  );
end entity;

architecture rtl of anon_types is
begin
end architecture;

package subprog_pkg is
  function f1 return integer;
  procedure p1;
  function f2 is new work.f1;
  procedure p2 is new work.p1;
end package;
