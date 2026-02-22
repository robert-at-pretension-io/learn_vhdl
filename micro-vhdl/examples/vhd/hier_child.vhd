library ieee;
use ieee.std_logic_1164.all;
entity hier_child is
  port (in1: in std_logic; out1: out std_logic);
end entity;
architecture rtl of hier_child is
begin
  out1 <= not in1;
end architecture;
