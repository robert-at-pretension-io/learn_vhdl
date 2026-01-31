library ieee;
use ieee.std_logic_1164.all;

entity sync_demo is
  port (
    clk_a    : in std_logic;
    clk_b    : in std_logic;
    async_in : in std_logic;
    sync_out : out std_logic
  );
end entity;

architecture rtl of sync_demo is
  signal async_sig : std_logic;
  signal meta1     : std_logic;
  signal meta2     : std_logic;
begin
  p_src : process(clk_a)
  begin
    if rising_edge(clk_a) then
      async_sig <= async_in;
    end if;
  end process;

  p_meta1 : process(clk_b)
  begin
    if rising_edge(clk_b) then
      meta1 <= async_sig;
    end if;
  end process;

  p_meta2 : process(clk_b)
  begin
    if rising_edge(clk_b) then
      meta2 <= meta1;
    end if;
  end process;

  p_out : process(clk_b)
  begin
    if rising_edge(clk_b) then
      sync_out <= meta2;
    end if;
  end process;
end architecture;
