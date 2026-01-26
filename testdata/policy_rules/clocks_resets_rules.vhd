library ieee;
use ieee.std_logic_1164.all;

entity clocks_resets_rules is
  port (
    clk_vec : in std_logic_vector(1 downto 0);
    clk_aux : in std_logic;
    rst_vec : in std_logic_vector(1 downto 0);
    rst     : in std_logic;
    data_in : in std_logic;
    q1      : out std_logic;
    q2      : out std_logic;
    q3      : out std_logic
  );
end clocks_resets_rules;

architecture rtl of clocks_resets_rules is
begin
  p_multi_clk: process(clk_vec, clk_aux)
  begin
    if rising_edge(clk_vec(0)) then
      q1 <= data_in;
    end if;
  end process;

  p_no_reset: process(clk_aux)
  begin
    if rising_edge(clk_aux) then
      q2 <= data_in;
    end if;
  end process;

  p_async_reset: process(clk_aux, rst)
  begin
    if rst = '1' then
      q3 <= '0';
    elsif rising_edge(clk_aux) then
      q3 <= data_in;
    end if;
  end process;
end rtl;
