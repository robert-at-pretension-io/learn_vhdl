entity construct_ready_valid_sequential is
  port (
    clk   : in std_logic;
    rst   : in std_logic;
    valid : in std_logic;
    ready : out std_logic
  );
end entity;

architecture rtl of construct_ready_valid_sequential is
begin
  verification : block
  begin
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      ready <= '0';
    elsif rising_edge(clk) then
      ready <= valid;
    end if;
  end process;
end architecture;
