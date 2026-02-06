library ieee;
use ieee.std_logic_1164.all;

entity child is
  port (
    Input : in std_logic_vector(0 downto 0)
  );
end entity;

architecture rtl of child is
begin
end architecture;

entity assoc_top is
end entity;

architecture rtl of assoc_top is
  signal s : std_logic;
begin
  u0 : entity work.child
    port map (
      Input(0) => s
    );
end architecture;
