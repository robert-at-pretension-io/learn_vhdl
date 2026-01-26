library ieee;
use ieee.std_logic_1164.all;

entity BadEntity is
  port (
    data   : in std_logic;
    result : out std_logic
  );
end BadEntity;

architecture rtl of BadEntity is
  signal n_reset : std_logic;
begin
  result <= data;
end rtl;
