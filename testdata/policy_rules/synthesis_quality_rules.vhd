-- Positive fixture for Batch 3: synthesis & quality rules
-- Each construct triggers one specific rule.

library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;
use ieee.std_logic_unsigned.all;  -- numeric_std_unsigned_mixing

entity synthesis_quality_ent is
  port (
    clk   : in  std_logic;
    rst_n : in  std_logic;
    sel   : in  std_logic_vector(1 downto 0);
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity synthesis_quality_ent;

architecture rtl of synthesis_quality_ent is
  signal data_reg : std_logic;
  signal counter  : unsigned(7 downto 0);
begin

  -- redundant_others_only: case with only 'when others =>'
  only_others_proc : process(sel)
  begin
    case sel is
      when others =>
        dout <= '0';
    end case;
  end process only_others_proc;

  -- sequential_signal_no_reset: sequential process without reset
  no_reset_proc : process(clk)
  begin
    if rising_edge(clk) then
      data_reg <= din;
    end if;
  end process no_reset_proc;

end architecture rtl;
