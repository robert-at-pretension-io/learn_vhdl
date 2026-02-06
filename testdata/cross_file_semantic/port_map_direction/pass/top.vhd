library ieee;
use ieee.std_logic_1164.all;

entity top is
  port (out_sig : out std_logic);
end entity;

architecture rtl of top is
begin
  u1: entity work.child
    port map (out_sig => out_sig);
end architecture;
