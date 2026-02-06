library ieee;
use ieee.std_logic_1164.all;

entity alias_object is
end entity;

architecture rtl of alias_object is
  signal s : std_logic;
  alias A : std_logic is s;
begin
  p : process
  begin
    A <= '1';
    wait;
  end process;
end architecture;
