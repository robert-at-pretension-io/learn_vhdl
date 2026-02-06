-- Positive fixture for dead code detection rules.
-- Each construct below should trigger its corresponding rule.

library ieee;
use ieee.std_logic_1164.all;

entity dead_code_ent is
  generic (
    DEAD_GEN : integer := 0
  );
  port (
    clk   : in  std_logic;
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity dead_code_ent;

architecture rtl of dead_code_ent is

  -- unused_constant: declared but never referenced
  constant DEAD_WIDTH : integer := 42;

  -- unused_type: declared but never used by any signal/port/var
  type dead_state_t is (IDLE, ACTIVE, DONE);

  -- unused_subtype: declared but never used
  subtype dead_range is integer range 0 to 15;

  -- unused_component: declared but never instantiated
  component dead_comp is
    port (
      a : in  std_logic;
      b : out std_logic
    );
  end component;

  -- record_field_unused: record with an unreferenced field
  type my_rec_t is record
    used_f   : std_logic;
    unused_f : std_logic;
  end record;

  -- write_only_signal: assigned but never read, not in port map
  signal write_only_reg : std_logic;

  -- Normal signal (used properly, should not trigger)
  signal data_reg : std_logic;
  signal rec_sig  : my_rec_t;

  -- unused_subprogram: function declared but never called
  function dead_func(x : std_logic) return std_logic is
  begin
    return not x;
  end function;

  -- unused_subprogram: procedure declared but never called
  procedure dead_proc(signal s : out std_logic) is
  begin
    s <= '0';
  end procedure;

begin

  seq_proc : process(clk)
  begin
    if rising_edge(clk) then
      data_reg       <= din;
      write_only_reg <= din;
      rec_sig.used_f <= din;
    end if;
  end process;

  dout <= data_reg;

end architecture rtl;
