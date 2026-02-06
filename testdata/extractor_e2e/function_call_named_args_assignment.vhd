library ieee;
use ieee.std_logic_1164.all;

entity named_args_assign is
end entity;

architecture rtl of named_args_assign is
  signal a : integer := 0;
  signal b : integer := 1;
  signal c : integer := 2;
begin
  p_main : process
    variable x : integer := 0;
  begin
    x := update_options(
      Param => a,
      Count => b,
      Mode  => c
    );
    wait;
  end process;
end architecture;
