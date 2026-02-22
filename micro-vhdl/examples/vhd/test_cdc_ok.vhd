-- CDC golden test: 50 MHz / 200 MHz two-domain design.
-- clk_slow domain produces data_reg; a sync_2ff instance bridges the
-- single bit to clk_fast domain. The fast process reads only the
-- synchronized output (data_sync), so no CDC violation occurs.
library ieee;
use ieee.std_logic_1164.all;

entity test_cdc_ok is
  port (
    clk_slow  : in std_logic;
    clk_fast  : in std_logic;
    data_in   : in std_logic;
    result    : out std_logic
  );
end entity;

architecture rtl of test_cdc_ok is
  signal data_reg  : std_logic;
  signal data_sync : std_logic;
begin

  -- Slow domain: latch the incoming data.
  process(clk_slow) begin
    if rising_edge(clk_slow) then
      data_reg <= data_in;
    end if;
  end process;

  -- CDC crossing: synchronize data_reg into the fast domain.
  u_sync : entity work.sync_2ff
    port map (
      clk_dst => clk_fast,
      d_in    => data_reg,
      d_out   => data_sync
    );

  -- Fast domain: use only the synchronized signal.
  process(clk_fast) begin
    if rising_edge(clk_fast) then
      result <= data_sync;
    end if;
  end process;

end architecture;
