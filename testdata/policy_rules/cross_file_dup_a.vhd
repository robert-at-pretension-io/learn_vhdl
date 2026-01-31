library ieee;
use ieee.std_logic_1164.all;

package dup_pkg is
  constant WIDTH : integer := 8;
end dup_pkg;

entity dup_ent is
  port (
    a : in std_logic;
    y : out std_logic
  );
end dup_ent;

architecture rtl of dup_ent is
begin
  y <= a;
end rtl;
