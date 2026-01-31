library ieee;
use ieee.std_logic_1164.all;

entity trailing_ports is
  port (
    a : in std_logic;
    b : out std_logic;
  );
end entity;

architecture rtl of trailing_ports is
begin
end architecture;

entity evt_demo is
  port (
    clk : in std_logic;
    rst : in std_logic;
    q   : out std_logic
  );
end entity;

architecture rtl_evt of evt_demo is
  signal d : std_logic;
begin
  p_evt : process(clk, rst)
  begin
    if rst = '0' then
      q <= '0';
    elsif clk'event and clk = '1' then
      q <= d;
    end if;
  end process;
end architecture;
