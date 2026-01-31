library ieee;
use ieee.std_logic_1164.all;

entity lit_demo is
  port (
    a : in std_logic_vector(7 downto 0);
    y : out std_logic_vector(7 downto 0)
  );
end entity;

architecture rtl of lit_demo is
  signal s : std_logic_vector(7 downto 0);
begin
  s <= 16#FF#;
  y <= std_logic_vector'(others => '0');
end architecture;
