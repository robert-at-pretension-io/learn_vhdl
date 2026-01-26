library ieee;
use ieee.std_logic_1164.all;

entity clean_rules is
  port (
    data_i : in std_logic;
    en_i   : in std_logic;
    data_o : out std_logic
  );
end entity clean_rules;

architecture rtl of clean_rules is
  signal data_s : std_logic;
begin
  comb_p : process(data_i, en_i)
  begin
    if en_i = '1' then
      data_s <= data_i;
    else
      data_s <= '0';
    end if;
  end process comb_p;

  data_o <= data_s;
end architecture rtl;
