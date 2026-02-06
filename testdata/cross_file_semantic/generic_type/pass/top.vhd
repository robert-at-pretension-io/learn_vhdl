library ieee;
use ieee.std_logic_1164.all;

entity top is
end entity;

architecture rtl of top is
begin
  u1: entity work.child
    generic map (G_MODE => '1');
end architecture;
