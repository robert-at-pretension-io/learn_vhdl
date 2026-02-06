entity construct_counter_false_positive is
  port (
    clk     : in std_logic;
    rst     : in std_logic;
    load_en : in std_logic;
    data_in : in integer
  );
end entity;

architecture rtl of construct_counter_false_positive is
  signal data_reg : integer;
begin
  verification : block
  begin
  end block verification;

  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      data_reg <= 0;
    elsif rising_edge(clk) then
      if load_en = '1' then
        data_reg <= data_in;
      else
        data_reg <= data_reg;
      end if;
    end if;
  end process;
end architecture;
