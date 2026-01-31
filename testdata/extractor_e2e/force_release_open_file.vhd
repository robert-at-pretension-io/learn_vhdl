library ieee;
use ieee.std_logic_1164.all;

entity fr_demo is
  port (clk : in std_logic);
end entity;

architecture rtl of fr_demo is
  signal x, y : std_logic;
  file f : text open read_mode is "foo.txt";
begin
  x <= y after 5 ns;
  x <= transport y after 10 ns;
  x <= force '1';
  x <= release;

  u1: entity work.child
    port map (clk => clk, d => open, q => x);
end architecture;
