library ieee;
use ieee.std_logic_1164.all;

entity ext_demo is
  port (
    a : in std_logic
  );
end entity;

architecture rtl of ext_demo is
  signal b : std_logic;
begin
  blk1 : block
    signal bx : std_logic;
  begin
    bx <= a after 5 ns;
    b <= bx;
  end block;

  p_ext : process(a)
  begin
    if << signal .tb.dut.ext_sig : std_logic >> = '1' then
      b <= a;
    end if;
  end process;
end architecture;
