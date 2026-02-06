library ieee;
use ieee.std_logic_1164.all;

entity calls_condition_selected is
end entity;

architecture rtl of calls_condition_selected is
  type ctrl_t is record
    cnt : std_logic;
  end record;
  signal ctrl : ctrl_t;
  type bus_req_t is record
    addr : std_logic_vector(3 downto 0);
  end record;
  signal bus_req_i : bus_req_t;
  signal done : std_logic;
begin
  p_cond: process(ctrl)
  begin
    if or_reduce_f(ctrl.cnt) = '1' then
      done <= '1';
    else
      done <= '0';
    end if;
    if bus_req_i.addr(2) = '1' then
      done <= '1';
    end if;
  end process;
end architecture;
