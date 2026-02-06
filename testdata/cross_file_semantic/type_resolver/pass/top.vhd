library ieee;
use ieee.std_logic_1164.all;
use work.pkg_a.all;

entity top is
end entity;

architecture rtl of top is
  signal sig : rec_t;
begin
  u1: entity work.child
    port map (data => sig);
end architecture;
