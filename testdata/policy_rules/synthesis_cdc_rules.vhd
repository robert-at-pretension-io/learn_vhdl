library ieee;
use ieee.std_logic_1164.all;

entity synthesis_cdc_rules is
  port (
    clk_a   : in std_logic;
    clk_b   : in std_logic;
    in_a    : in std_logic;
    out_c   : out std_logic;
    out_comb: out std_logic
  );
end synthesis_cdc_rules;

architecture rtl of synthesis_cdc_rules is
  type array_mem_t is array (0 to 15) of std_logic_vector(7 downto 0);
  signal cross_bit  : std_logic;
  signal cross_bus  : std_logic_vector(3 downto 0);
  signal cross_sync : std_logic;
  signal sync1      : std_logic;
  signal clk_gate   : std_logic;
  signal bus_sink   : std_logic_vector(3 downto 0);
  signal huge_bus   : std_logic_vector(127 downto 0);
  signal mem_array  : array_mem_t;
  signal ready      : std_logic;
  signal sync_sink  : std_logic;
  signal gate_ff    : std_logic;
begin
  clk_gate <= clk_a and in_a;

  proc_a: process(clk_a)
  begin
    if rising_edge(clk_a) then
      cross_bit <= in_a;
      cross_bus <= (others => in_a);
      cross_sync <= in_a;
    end if;
  end process;

  proc_b: process(clk_b)
  begin
    if rising_edge(clk_b) then
      out_c <= cross_bit;
      bus_sink <= cross_bus;
      ready <= cross_bit;
      sync_sink <= cross_sync;
    end if;
  end process;

  sync_stage: process(clk_b)
  begin
    if rising_edge(clk_b) then
      sync1 <= cross_sync;
    end if;
  end process;

  gated_proc: process(clk_gate)
  begin
    if rising_edge(clk_gate) then
      gate_ff <= in_a;
    end if;
  end process;

  comb_proc: process(in_a)
  begin
    out_comb <= in_a;
  end process;
end rtl;
