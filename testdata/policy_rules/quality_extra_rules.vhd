-- Positive fixture for extra quality rules.

library ieee;
use ieee.std_logic_1164.all;
use ieee.std_logic_1164.all;
use ieee.math_real.all;
use work.my_custom_pkg.all;

entity quality_extra_ent is
  port (
    clk  : in  std_logic;
    din  : in  std_logic;
    dout : out std_logic
  );
end entity quality_extra_ent;

architecture rtl of quality_extra_ent is
  signal hot_sig : std_logic;
begin

  -- duplicate_use_clause: ieee.std_logic_1164.all appears twice above
  -- use_all_abuse: work.my_custom_pkg.all is non-standard .all import
  -- unused_use_clause: ieee.math_real.all has no math symbols used

  -- signal_fanout: hot_sig read by 12 processes
  p0:  process(hot_sig) begin dout <= hot_sig; end process;
  p1:  process(hot_sig) begin dout <= hot_sig; end process;
  p2:  process(hot_sig) begin dout <= hot_sig; end process;
  p3:  process(hot_sig) begin dout <= hot_sig; end process;
  p4:  process(hot_sig) begin dout <= hot_sig; end process;
  p5:  process(hot_sig) begin dout <= hot_sig; end process;
  p6:  process(hot_sig) begin dout <= hot_sig; end process;
  p7:  process(hot_sig) begin dout <= hot_sig; end process;
  p8:  process(hot_sig) begin dout <= hot_sig; end process;
  p9:  process(hot_sig) begin dout <= hot_sig; end process;
  p10: process(hot_sig) begin dout <= hot_sig; end process;

  driver: process(clk)
  begin
    if rising_edge(clk) then
      hot_sig <= din;
    end if;
  end process;

end architecture rtl;
