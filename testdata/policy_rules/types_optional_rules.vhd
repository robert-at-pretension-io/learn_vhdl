library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity types_optional_rules is
  port (
    clk : in std_logic
  );
end types_optional_rules;

architecture rtl of types_optional_rules is
  signal signed_sig : signed(7 downto 0);
  signal unsigned_sig : unsigned(7 downto 0);
begin
  null;
end rtl;
