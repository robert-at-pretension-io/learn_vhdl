library ieee;
use ieee.std_logic_1164.all;

entity loop_idx is
  port (
    clk  : in std_logic
  );
end entity;

architecture rtl of loop_idx is
  type vec_t is array (0 to 1) of std_logic_vector(7 downto 0);
  signal arr : vec_t;
  signal out_v : std_logic;
begin
  p1: process(clk)
    variable tmp : std_logic;
  begin
    for i in 0 to 1 loop
      tmp := or_reduce_f(arr(i)(7 downto 0));
    end loop;
    out_v <= tmp;
  end process;
end architecture;
