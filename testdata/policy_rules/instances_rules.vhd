library ieee;
use ieee.std_logic_1164.all;

entity child is
  port (
    a : in std_logic;
    y : out std_logic
  );
end child;

architecture rtl of child is
begin
  y <= a;
end rtl;

entity top is
  port (
    a : in std_logic;
    y : out std_logic
  );
end top;

architecture rtl of top is
begin
  u1: entity work.child;
end rtl;
