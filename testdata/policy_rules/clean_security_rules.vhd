library ieee;
use ieee.std_logic_1164.all;

entity clean_security_rules is
  port (
    data_i : in std_logic;
    data_o : out std_logic
  );
end entity clean_security_rules;

architecture rtl of clean_security_rules is
begin
  comb_p : process(data_i)
  begin
    if data_i = '1' then
      data_o <= '1';
    else
      data_o <= '0';
    end if;
  end process comb_p;
end architecture rtl;
