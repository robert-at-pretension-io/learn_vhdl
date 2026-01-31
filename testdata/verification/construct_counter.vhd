library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity construct_counter is
  port (
    clk : in std_logic;
    rst : in std_logic
  );
end entity;

architecture rtl of construct_counter is
  signal idx : unsigned(3 downto 0);
begin
  verification : block
  begin
  end block verification;

  p_cnt : process(clk, rst)
  begin
    if rst = '1' then
      idx <= (others => '0');
    elsif rising_edge(clk) then
      idx <= idx + 1;
    end if;
  end process;
end architecture;
