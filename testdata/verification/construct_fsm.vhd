entity construct_fsm is
  port (
    clk : in std_logic;
    rst : in std_logic
  );
end entity;

architecture rtl of construct_fsm is
  type mode_t is (M0, M1, M2);
  signal mode : mode_t;
begin
  verification : block
  begin
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      mode <= M0;
    elsif rising_edge(clk) then
      case mode is
        when M0 =>
          mode <= M1;
        when M1 =>
          mode <= M2;
        when others =>
          mode <= M0;
      end case;
    end if;
  end process;
end architecture;
