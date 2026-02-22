library IEEE;
use IEEE.std_logic_1164.all;

-- Simple pass-through with contract:
-- require: a = '0' (input is always 0)
-- ensure:  z = '0' (output mirrors input, so also always 0)
entity pass_through is
  port (
    clk : in std_logic;
    a   : in std_logic;
    z   : out std_logic
  );
  contract
    require: a = '0';
    ensure:  z = '0';
end entity pass_through;

architecture behavior of pass_through is
begin
  z <= a;
end architecture behavior;
