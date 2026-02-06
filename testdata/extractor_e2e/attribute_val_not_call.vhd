library ieee;
use ieee.std_logic_1164.all;

entity attr_val_tb is
end entity;

architecture rtl of attr_val_tb is
  type state_t is (IDLE, BUSY);
  signal state_i : integer := 0;
  signal state_e : state_t := IDLE;
begin
  p_attr : process
    variable v : state_t;
  begin
    v := state_t'val(state_i);
    state_e <= v;
    wait;
  end process;
end architecture;
