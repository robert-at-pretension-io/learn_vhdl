library ieee;
use ieee.std_logic_1164.all;

entity unlabeled_gen is
  port (
    a : in std_logic;
    y : out std_logic
  );
end unlabeled_gen;

architecture rtl of unlabeled_gen is
  signal s : std_logic;
begin
  s <= a;
  for i in 0 to 0 generate
    y <= s;
  end generate;
end rtl;
