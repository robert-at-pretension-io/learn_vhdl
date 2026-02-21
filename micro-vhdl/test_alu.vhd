library IEEE;
use IEEE.std_logic_1164.all;

entity alu is
  port (
    opcode : in std_logic_vector(1 downto 0);
    a, b   : in std_logic_vector(7 downto 0);
    res    : out std_logic_vector(7 downto 0)
  );
end entity alu;

architecture behavior of alu is
begin
  with opcode select
    res <= a + b when "00",
           a - b when "01",
           a and b when "10",
           a xor b when others;
end architecture behavior;