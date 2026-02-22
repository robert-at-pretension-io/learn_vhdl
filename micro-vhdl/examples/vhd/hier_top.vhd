library ieee;
use ieee.std_logic_1164.all;
entity hier_top is
  port (top_in: in std_logic; top_out: out std_logic);
end entity;
architecture rtl of hier_top is
begin
  u_child: entity work.hier_child port map (in1 => top_in, out1 => top_out);
  psl assert always top_in = '1' |-> top_out = '0';
end architecture;
