entity waived_cover_companion is
  port (
    clk : in std_logic;
    rst : in std_logic
  );
end entity;

architecture rtl of waived_cover_companion is
  type state_t is (IDLE, RUN);
  signal state : state_t;
begin
  verification : block
  begin
    --@check id=fsm.legal_state scope=arch:rtl state=state
    --@waive id=missing_cover_companion scope=arch:rtl reason="legacy cover elsewhere" owner="lint"
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      state <= IDLE;
    elsif rising_edge(clk) then
      case state is
        when IDLE => state <= RUN;
        when RUN  => state <= IDLE;
        when others => state <= IDLE;
      end case;
    end if;
  end process;
end architecture;
