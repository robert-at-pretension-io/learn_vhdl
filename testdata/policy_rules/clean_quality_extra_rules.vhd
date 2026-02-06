-- Negative fixture for extra quality rules.
-- Clean code that should NOT trigger duplicate_use_clause, use_all_abuse,
-- signal_fanout, unused_library_clause, or unused_use_clause.

library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity clean_quality_extra_ent is
  port (
    clk  : in  std_logic;
    din  : in  std_logic_vector(7 downto 0);
    dout : out unsigned(7 downto 0)
  );
end entity clean_quality_extra_ent;

architecture rtl of clean_quality_extra_ent is
  signal data_reg : unsigned(7 downto 0);
begin

  -- Only one process reads data_reg (well below fanout threshold)
  p0: process(clk)
  begin
    if rising_edge(clk) then
      data_reg <= unsigned(din);
    end if;
  end process;

  dout <= data_reg;

end architecture rtl;
