library IEEE;
use IEEE.std_logic_1164.all;

-- Deliberate contract violation:
-- This module outputs NOT(a), but the contract claims z = a.
-- The ensure condition is false whenever a = '0' and z = '1'.
entity invert_wrong_contract is
  port (
    clk : in std_logic;
    a   : in std_logic;
    z   : out std_logic
  );
  contract
    ensure: z = a;
end entity invert_wrong_contract;

architecture behavior of invert_wrong_contract is
begin
  z <= not a;
end architecture behavior;
