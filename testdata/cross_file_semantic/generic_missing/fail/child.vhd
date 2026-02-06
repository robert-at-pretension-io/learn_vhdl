library ieee;
use ieee.std_logic_1164.all;

entity child is
  generic (G_WIDTH : integer);
  port (data : out std_logic_vector(G_WIDTH-1 downto 0));
end entity;

architecture rtl of child is
begin
  data <= (others => '0');
end architecture;
