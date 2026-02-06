library ieee;
use ieee.std_logic_1164.all;
use work.pkg.all;

entity top is
end entity;

architecture rtl of top is
  signal vec : std_logic_vector(3 downto 0);
  signal y : integer := 0;
begin
  process(all)
  begin
    y <= f(1);
    y <= f(vec);
  end process;
end architecture;
