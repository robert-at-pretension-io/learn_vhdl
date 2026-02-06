-- Negative fixture for variable analysis rules.
-- Clean code that should NOT trigger any variable rules.

library ieee;
use ieee.std_logic_1164.all;

entity clean_variable_ent is
  port (
    clk   : in  std_logic;
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity clean_variable_ent;

architecture rtl of clean_variable_ent is
begin

  good_proc : process(clk)
    variable temp : std_logic;
  begin
    if rising_edge(clk) then
      temp := din;
      dout <= temp;
    end if;
  end process;

end architecture rtl;
