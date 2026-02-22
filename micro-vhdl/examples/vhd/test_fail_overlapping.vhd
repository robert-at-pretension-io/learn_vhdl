entity delay_mismatch is
  port (req, clk : in std_logic; ack : out std_logic);
end entity;

architecture rtl of delay_mismatch is
begin
  process(clk) begin
    if rising_edge(clk) then
      ack <= req; -- 1 cycle delay
    end if;
  end process;

  -- Overlapping implication requires ack to be true in the SAME cycle as req.
  -- This will fail immediately because ack takes 1 cycle to propagate (req |=> ack would pass).
  psl assert always req |-> ack;
end architecture;
