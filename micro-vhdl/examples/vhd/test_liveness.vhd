entity test_live_pass is
  port (req : in std_logic; ack : out std_logic; clk : in std_logic);
end entity;

architecture rtl of test_live_pass is
begin
  ack <= req;
  psl assert always (req -> eventually! ack);
end architecture;
