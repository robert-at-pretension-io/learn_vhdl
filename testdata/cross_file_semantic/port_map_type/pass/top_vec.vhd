library ieee;
use ieee.std_logic_1164.all;

entity top_vec is
end entity;

architecture rtl of top_vec is
begin
  u1: entity work.child_vec
    port map (data => std_logic_vector'(others => '0'));
end architecture;
