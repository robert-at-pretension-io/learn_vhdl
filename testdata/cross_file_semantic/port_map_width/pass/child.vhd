library ieee;
use ieee.std_logic_1164.all;

entity child is
  port (data : out std_logic_vector(7 downto 0));
end entity;

architecture rtl of child is
begin
  data <= (others => '0');
end architecture;
