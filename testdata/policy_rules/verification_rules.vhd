entity verification_rules is
  port (
    clk   : in std_logic;
    rst   : in std_logic;
    valid : in std_logic;
    ready : in std_logic
  );
end entity;

architecture rtl of verification_rules is
  type state_t is (S_IDLE, S_RUN);
  signal state : state_t;
begin
  verification : block
  begin
    --@check id=fsm.legal_state scope=arch:rtl state=state
    --@check id=fsm.reset_known scope=arch:rtl
    --@check id=fsm.legal_state scope=arch:rtl state=missing_state
    --@check id=rv.eventual_progress_bounded scope=arch:rtl valid=valid ready=ready
    --@waive id=missing_cover_companion scope=arch:rtl
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      state <= S_IDLE;
    elsif rising_edge(clk) then
      case state is
        when S_IDLE =>
          state <= S_RUN;
        when others =>
          state <= S_IDLE;
      end case;
    end if;
  end process;
end architecture;
