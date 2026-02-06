library ieee;
use ieee.std_logic_1164.all;

entity child is
  port (out_sig : out std_logic);
end entity;

architecture rtl of child is
begin
  out_sig <= '0';
end architecture;
