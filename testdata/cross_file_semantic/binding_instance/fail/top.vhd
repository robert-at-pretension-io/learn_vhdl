library ieee;
use ieee.std_logic_1164.all;

entity top is
  port (a : in std_logic);
end entity;

architecture rtl of top is
begin
  u1: entity work.missing
    port map (a => a);
end architecture;
