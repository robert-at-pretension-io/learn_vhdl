entity tag_parsing_fixture is
  port (
    clk   : in std_logic;
    rst   : in std_logic;
    valid : in std_logic;
    ready : in std_logic;
    gnt_a : in std_logic;
    gnt_b : in std_logic;
    gnt_c : in std_logic
  );
end entity;

architecture rtl of tag_parsing_fixture is
  type state_t is (S0, S1);
  signal state : state_t;
begin
  --@check id=fsm.legal_state scope=arch:rtl state=outside

  verification : block
  begin
    --@check id=rv.stable_while_stalled scope=arch:rtl valid="valid signal" ready=ready
    --@check id=arb.onehot0 scope=arch:rtl grants="gnt_a,gnt_b,gnt_c"
    --@check id=fsm.legal_state scope=arch:rtl state=state note="comma, list"
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      state <= S0;
    elsif rising_edge(clk) then
      state <= S1;
    end if;
  end process;
end architecture;
