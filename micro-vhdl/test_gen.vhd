library IEEE;
use IEEE.std_logic_1164.all;

entity my_module is
  port (
    a : in std_logic_vector(3 downto 0);
    out_sig : out std_logic_vector(3 downto 0)
  );
end entity my_module;

architecture behavior of my_module is
begin
  gen_loop : for i in 0 to 3 generate
    out_sig <= a;
  end generate;
end architecture behavior;