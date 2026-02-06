entity stale_binding_fixture is
  port (
    clk : in std_logic;
    rst : in std_logic
  );
end entity;

architecture rtl of stale_binding_fixture is
  type state_t is (IDLE, RUN);
  signal rx_state_q : state_t;
begin
  verification : block
  begin
    --@check id=fsm.legal_state scope=arch:rtl state=rx_state
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      rx_state_q <= IDLE;
    elsif rising_edge(clk) then
      case rx_state_q is
        when IDLE =>
          rx_state_q <= RUN;
        when others =>
          rx_state_q <= IDLE;
      end case;
    end if;
  end process;
end architecture;
