library ieee;
use ieee.std_logic_1164.all;
use work.pkg.all;

entity top is
end entity;

architecture rtl of top is
  signal s : std_logic := '0';
  signal i : integer := 3;
  signal y : integer := 0;
begin
  process(all)
  begin
    y <= f(i);
    y <= f(s);
  end process;
end architecture;
