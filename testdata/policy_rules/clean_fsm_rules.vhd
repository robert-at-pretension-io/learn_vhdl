library ieee;
use ieee.std_logic_1164.all;

entity clean_fsm_rules is
  port (
    clk_i : in std_logic;
    rst_n : in std_logic
  );
end entity clean_fsm_rules;

architecture rtl of clean_fsm_rules is
  type state_t is (S_IDLE, S_RUN, S_DONE);
  signal state : state_t;
  signal next_state : state_t;
begin
  fsm_p : process(clk_i, rst_n)
  begin
    if rst_n = '0' then
      state <= S_IDLE;
      next_state <= S_IDLE;
    elsif rising_edge(clk_i) then
      case state is
        when S_IDLE => state <= S_RUN;
        when S_RUN => state <= S_DONE;
        when S_DONE => state <= S_IDLE;
        when others => state <= S_IDLE;
      end case;
      next_state <= state;
    end if;
  end process fsm_p;
end architecture rtl;
