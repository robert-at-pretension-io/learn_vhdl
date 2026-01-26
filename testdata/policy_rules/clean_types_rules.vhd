library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity clean_types_rules is
  port (
    data_i : in unsigned(3 downto 0);
    data_o : out unsigned(3 downto 0)
  );
end entity clean_types_rules;

architecture rtl of clean_types_rules is
  signal data_s : unsigned(3 downto 0);
begin
  data_s <= data_i;
  data_o <= data_s;
end architecture rtl;
