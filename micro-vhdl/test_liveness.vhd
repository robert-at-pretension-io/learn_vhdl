entity test_live is
  port (req, ack : in std_logic; clk : in std_logic);
end entity;

architecture rtl of test_live is
begin
  psl assert always (req -> eventually! ack);
end architecture;
