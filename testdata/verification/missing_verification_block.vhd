entity missing_verification_block is
  port (
    clk   : in std_logic;
    reset : in std_logic
  );
end entity;

architecture rtl of missing_verification_block is
  type state_t is (S_IDLE, S_RUN);
  signal state : state_t;
begin
  process(clk, reset)
  begin
    if reset = '1' then
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
