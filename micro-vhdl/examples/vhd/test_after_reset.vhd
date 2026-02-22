library IEEE;
use IEEE.std_logic_1164.all;

-- Demonstrates psl assert always after_reset(rst).
--
-- A register that is SET to '1' on reset and holds its value forever.
-- Property: after reset has de-asserted, the register is always '1'.
--
-- Without after_reset this property would fail in IC3 because the register
-- starts unconstrained (could be '0'). With after_reset, BMC correctly
-- observes that reset initialises the register to '1' before the check begins.
entity sticky_high is
  port (
    clk : in std_logic;
    rst : in std_logic;
    s   : out std_logic
  );
end entity sticky_high;

architecture behavior of sticky_high is
  signal s_sig : std_logic;
begin
  process(clk)
  begin
    if rising_edge(clk) then
      if rst = '1' then
        s_sig <= '1';   -- reset drives the register HIGH
      end if;
      -- no else: holds its value forever once set
    end if;
  end process;

  s <= s_sig;

  -- After reset has fired at least once, s_sig is always '1'.
  psl assert always after_reset(rst) s_sig;
end architecture behavior;
