entity Test is
  port (clk : in std_logic; req : in std_logic; ack : out std_logic);
end entity;

architecture rtl of Test is
begin
  process(clk) begin
    if rising_edge(clk) then
      ack <= req;
    end if;
  end process;

  psl cover (req = '1' and ack = '1');
  psl assert never (req = '0' and ack = '1');
end architecture;