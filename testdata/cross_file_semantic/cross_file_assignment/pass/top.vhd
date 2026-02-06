library ieee;
use ieee.std_logic_1164.all;

entity top is
end entity;

architecture rtl of top is
  signal sig : integer;
begin
  u1: entity work.child
    port map (out_sig => sig);
end architecture;
