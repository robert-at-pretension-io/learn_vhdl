library IEEE;
use IEEE.std_logic_1164.all;

entity next_delay_fail is
  port (
    clk : in std_logic;
    req : in std_logic;
    ack : out std_logic
  );
end entity next_delay_fail;

architecture behavior of next_delay_fail is
  signal d1, d2, d3 : std_logic;
begin
  process(clk)
  begin
    if rising_edge(clk) then
      d1 <= req;
      d2 <= d1;
      d3 <= d2;
      ack <= d3;
    end if;
  end process;

  -- This is intentionally wrong: ack is delayed by 3 cycles, not 1-2.
  psl assert always req |=> next[2](ack);
  psl assert always req |=> next_a[1 to 2](ack);
end architecture behavior;
