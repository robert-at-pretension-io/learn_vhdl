-- Negative fixture for dead code detection rules.
-- Clean code that should NOT trigger any of the dead code rules.

library ieee;
use ieee.std_logic_1164.all;

entity clean_dead_code_ent is
  generic (
    WIDTH : integer := 8
  );
  port (
    clk   : in  std_logic;
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity clean_dead_code_ent;

architecture rtl of clean_dead_code_ent is

  -- Constant that IS referenced
  constant DATA_WIDTH : integer := WIDTH;

  -- Type that IS used by a signal
  type state_t is (IDLE, RUN);

  -- Subtype that IS used
  subtype cnt_range is integer range 0 to 255;

  -- Component that IS instantiated
  component sub_mod is
    port (
      a : in  std_logic;
      b : out std_logic
    );
  end component;

  -- Record with all fields used
  type my_rec_t is record
    valid : std_logic;
    data  : std_logic;
  end record;

  -- Signal that is both written and read
  signal data_reg  : std_logic;
  signal state_sig : state_t;
  signal cnt       : cnt_range;
  signal rec_sig   : my_rec_t;

  -- Function that IS called
  function invert(x : std_logic) return std_logic is
  begin
    return not x;
  end function;

  -- Procedure that IS called
  procedure clear_reg(signal s : out std_logic) is
  begin
    s <= '0';
  end procedure;

  signal sub_out : std_logic;

begin

  -- Use the constant and generic in a comparison
  seq_proc : process(clk)
  begin
    if rising_edge(clk) then
      if cnt < DATA_WIDTH then
        cnt := cnt + 1;
      end if;
      data_reg       <= invert(din);
      state_sig      <= RUN;
      rec_sig.valid  <= '1';
      rec_sig.data   <= din;
    end if;
  end process;

  ctrl_proc : process(clk)
  begin
    if rising_edge(clk) then
      clear_reg(dout);
    end if;
  end process;

  -- Read data_reg, state_sig, and rec_sig
  dout <= data_reg when state_sig = IDLE else rec_sig.valid;

  -- Instantiate the component
  u_sub : sub_mod
    port map (
      a => din,
      b => sub_out
    );

end architecture rtl;
