library IEEE;
use IEEE.std_logic_1164.all;

-- Test psl assume: constrain req to never deassert once high,
-- then prove that ack follows req one cycle later.
-- Without the assume, req is adversarial and Z3 finds a trivial CEX.
entity handshake is
  port (
    clk : in std_logic;
    req : in std_logic;
    ack : out std_logic
  );
end entity handshake;

architecture behavior of handshake is
  signal ack_sig : std_logic;
begin
  ack_sig <= req;

  process(clk)
  begin
    if rising_edge(clk) then
      ack <= ack_sig;
    end if;
  end process;

  -- Constrain the environment: req once asserted stays high.
  psl assume always req |=> req;

  -- Property: ack follows req with one cycle latency.
  psl assert always req |=> ack;
end architecture behavior;
