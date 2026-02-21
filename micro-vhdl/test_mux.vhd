library IEEE;
use IEEE.std_logic_1164.all;

entity my_module is
  port (
    clk : in std_logic;
    a, b : in std_logic;
    out_sig : out std_logic
  );
end entity my_module;

architecture behavior of my_module is
  signal temp : std_logic;
begin
  temp <= a and b;

  process(clk)
  begin
    if rising_edge(clk) then
      if a = '1' then
        out_sig <= '1';
      elsif b = '1' then
        out_sig <= '0';
      else
        out_sig <= temp;
      end if;
    end if;
  end process;

end architecture behavior;