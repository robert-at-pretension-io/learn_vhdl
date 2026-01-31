library ieee;
use ieee.std_logic_1164.all;

entity demo is
  port (
    clk   : in std_logic;
    rst_n : in std_logic;
    din   : in std_logic_vector(7 downto 0);
    dout  : out std_logic_vector(7 downto 0)
  );

  function add(a, b : integer) return integer;
  procedure touch(signal s : in std_logic);
end entity;

architecture rtl of demo is
  signal a, b : std_logic_vector(7 downto 0);
  signal flag : std_logic := '0';
begin
  p_sync : process(clk, rst_n)
  begin
    if rst_n = '0' then
      dout <= (others => '0');
    elsif rising_edge(clk) then
      dout <= din;
    end if;
  end process;
end architecture;
