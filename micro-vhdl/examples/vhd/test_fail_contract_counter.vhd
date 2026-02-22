entity bounded_counter is
  port (inc, clk : in std_logic; val : out std_logic_vector(3 downto 0));
  contract
    -- We assume the counter is used safely and never wraps around to x"F"
    -- But since we don't have an assumption restricting 'inc', it will fail!
    ensure: val /= x"f";
end entity;

architecture rtl of bounded_counter is
  signal count : std_logic_vector(3 downto 0);
begin
  process(clk) begin
    if rising_edge(clk) then
      if inc = '1' then
        count <= count + "0001";
      end if;
    end if;
  end process;
  val <= count;
end architecture;
