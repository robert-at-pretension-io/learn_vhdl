library IEEE;
use IEEE.std_logic_1164.all;

entity my_module is
  port (
    clk : in std_logic;
    req : in std_logic;
    ack : out std_logic
  );
end entity my_module;

architecture behavior of my_module is
  signal ack_sig : std_logic;
begin
  ack_sig <= req;
  
  process(clk)
  begin
    if rising_edge(clk) then
      ack <= ack_sig;
    end if;
  end process;

  psl assert always req |=> ack;
  psl assert always req -> eventually! ack;
  psl assert always stable(req);
end architecture behavior;