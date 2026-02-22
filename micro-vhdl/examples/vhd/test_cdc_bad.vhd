-- CDC violation test: same two-domain design as test_cdc_ok, but WITHOUT
-- the sync_2ff synchronizer. The clk_fast process reads data_reg directly
-- from the clk_slow domain — an illegal CDC crossing.
-- Expected: semantic error at the cross-domain read site, exit non-zero.
library ieee;
use ieee.std_logic_1164.all;

entity test_cdc_bad is
  port (
    clk_slow  : in std_logic;
    clk_fast  : in std_logic;
    data_in   : in std_logic;
    result    : out std_logic
  );
end entity;

architecture rtl of test_cdc_bad is
  signal data_reg : std_logic;
begin

  -- Slow domain: latch the incoming data.
  process(clk_slow) begin
    if rising_edge(clk_slow) then
      data_reg <= data_in;
    end if;
  end process;

  -- Fast domain: ILLEGAL — reads data_reg directly without a synchronizer.
  process(clk_fast) begin
    if rising_edge(clk_fast) then
      result <= data_reg;
    end if;
  end process;

end architecture;
