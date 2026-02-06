library ieee;
use ieee.std_logic_1164.all;

entity top is
end entity;

architecture rtl of top is
  signal sig : std_logic_vector(3 downto 0);
begin
  u1: entity work.child
    port map (data => sig);
end architecture;
