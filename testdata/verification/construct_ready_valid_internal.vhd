entity construct_ready_valid_internal is
  port (
    clk : in std_logic;
    rst : in std_logic
  );
end entity;

architecture rtl of construct_ready_valid_internal is
  signal valid_i : std_logic;
  signal ready_i : std_logic;
  signal xfer    : std_logic;
begin
  verification : block
  begin
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      valid_i <= '0';
      ready_i <= '0';
    elsif rising_edge(clk) then
      valid_i <= not valid_i;
      ready_i <= valid_i;
    end if;
  end process;

  xfer <= valid_i and ready_i;
end architecture;
