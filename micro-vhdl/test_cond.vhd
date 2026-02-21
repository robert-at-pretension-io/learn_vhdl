library IEEE;
use IEEE.std_logic_1164.all;

entity my_module is
  port (
    sel : in std_logic_vector(1 downto 0);
    a, b, c : in std_logic;
    out_sig : out std_logic;
    out2 : out std_logic
  );
end entity my_module;

architecture behavior of my_module is
begin
  out2 <= a when sel = "00" else b;

  with sel select
    out_sig <= a when "01",
               b when "10",
               c when others;

end architecture behavior;