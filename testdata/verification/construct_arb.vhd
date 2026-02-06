entity construct_arb is
  port (
    clk   : in std_logic;
    rst   : in std_logic;
    req_a : in std_logic;
    req_b : in std_logic;
    gnt_a : out std_logic;
    gnt_b : out std_logic
  );
end entity;

architecture rtl of construct_arb is
begin
  verification : block
  begin
  end block verification;

  p_arb : process(clk, rst)
  begin
    if rst = '1' then
      gnt_a <= '0';
      gnt_b <= '0';
    elsif rising_edge(clk) then
      if req_a = '1' then
        gnt_a <= '1';
        gnt_b <= '0';
      elsif req_b = '1' then
        gnt_a <= '0';
        gnt_b <= '1';
      else
        gnt_a <= '0';
        gnt_b <= '0';
      end if;
    end if;
  end process;
end architecture;
