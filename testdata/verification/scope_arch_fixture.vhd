entity scope_arch_fixture is
  port (
    clk : in std_logic;
    rst : in std_logic
  );
end entity;

architecture rtl of scope_arch_fixture is
  type mode_t is (A0, A1);
  signal mode : mode_t;
begin
  verification : block
  begin
    --@check id=fsm.legal_state scope=arch:rtl state=mode
    --@check id=fsm.reset_known scope=arch:rtl state=mode
    --@check id=cover.fsm.transition_taken scope=arch:rtl state=mode
  end block verification;

  p_rtl : process(clk, rst)
  begin
    if rst = '1' then
      mode <= A0;
    elsif rising_edge(clk) then
      case mode is
        when A0 =>
          mode <= A1;
        when others =>
          mode <= A0;
      end case;
    end if;
  end process;
end architecture;

architecture gate of scope_arch_fixture is
  type mode2_t is (B0, B1);
  signal mode2 : mode2_t;
begin
  verification : block
  begin
  end block verification;

  p_gate : process(clk, rst)
  begin
    if rst = '1' then
      mode2 <= B0;
    elsif rising_edge(clk) then
      case mode2 is
        when B0 =>
          mode2 <= B1;
        when others =>
          mode2 <= B0;
      end case;
    end if;
  end process;
end architecture;
