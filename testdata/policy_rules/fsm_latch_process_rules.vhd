library ieee;
use ieee.std_logic_1164.all;

entity fsm_latch_rules is
  port (
    a : in std_logic;
    clk : in std_logic;
    y : out std_logic
  );
end fsm_latch_rules;

architecture rtl of fsm_latch_rules is
  type state_t is (IDLE, RUN, DONE);
  signal state     : state_t;
  signal state_vec : std_logic_vector(1 downto 0);
  signal s0 : std_logic;
  signal s1 : std_logic;
  signal s2 : std_logic;
  signal s3 : std_logic;
  signal s4 : std_logic;
  signal s5 : std_logic;
  signal s6 : std_logic;
  signal s7 : std_logic;
  signal s8 : std_logic;
  signal s9 : std_logic;
  signal s10 : std_logic;
  signal s11 : std_logic;
  signal s12 : std_logic;
  signal s13 : std_logic;
  signal s14 : std_logic;
  signal s15 : std_logic;
  signal s16 : std_logic;
  signal s17 : std_logic;
  signal s18 : std_logic;
  signal s19 : std_logic;
  signal s20 : std_logic;
begin
  y <= a when a = '1' else '0';

  with state select s0 <= '1' when IDLE,
                       '0' when RUN;

  comb_proc: process(state, a)
  begin
    s0 <= a;
    s1 <= a;
    s2 <= a;
    s3 <= a;
    s4 <= a;
    s5 <= a;
    s6 <= a;
    s7 <= a;
    s8 <= a;
    s9 <= a;
    s10 <= a;
    s11 <= a;
    s12 <= a;
    s13 <= a;
    s14 <= a;
    s15 <= a;
    s16 <= a;
    s17 <= a;
    s18 <= a;
    s19 <= a;
    s20 <= a;
    case state is
      when IDLE => y <= '0';
      when RUN => y <= '1';
    end case;
  end process;

  seq_state: process(clk)
  begin
    if rising_edge(clk) then
      state <= IDLE;
    end if;
  end process;
end rtl;
