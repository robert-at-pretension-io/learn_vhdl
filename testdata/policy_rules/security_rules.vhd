library ieee;
use ieee.std_logic_1164.all;

entity security_rules is
  port (
    clk    : in std_logic;
    counter: in std_logic_vector(31 downto 0);
    flag1  : out std_logic;
    flag2  : out std_logic;
    flag3  : out std_logic
  );
end security_rules;

architecture rtl of security_rules is
begin
  p_triggers: process(clk)
  begin
    if rising_edge(clk) then
      flag1 <= (counter = X"DEADBEEF");
      flag2 <= (counter /= X"ABCD1234");
      flag3 <= (counter = X"CAFEBABE");
    end if;
  end process;
end rtl;
