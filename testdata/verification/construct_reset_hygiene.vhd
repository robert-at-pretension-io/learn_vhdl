entity construct_reset_hygiene is
  port (
    clk  : in std_logic;
    rst  : in std_logic;
    done : out std_logic
  );
end entity;

architecture rtl of construct_reset_hygiene is
  type state_t is (S0, S1);
  signal state  : state_t;
  signal count  : integer;
begin
  verification : block
  begin
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      state <= S0;
      count <= 0;
      done <= '0';
    elsif rising_edge(clk) then
      count <= count + 1;
      if count = 0 then
        state <= S1;
      end if;
      done <= '1';
    end if;
  end process;
end architecture;
