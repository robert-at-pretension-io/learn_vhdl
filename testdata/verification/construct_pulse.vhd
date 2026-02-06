entity construct_pulse is
  port (
    clk   : in std_logic;
    rst   : in std_logic;
    req   : in std_logic;
    pulse : out std_logic
  );
end entity;

architecture rtl of construct_pulse is
  signal req_d : std_logic;
begin
  verification : block
  begin
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      req_d <= '0';
    elsif rising_edge(clk) then
      req_d <= req;
    end if;
  end process;

  pulse <= req and not req_d;
end architecture;
