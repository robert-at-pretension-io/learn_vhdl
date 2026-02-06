library ieee;
use ieee.std_logic_1164.all;

entity verification_scope_mismatch is
end entity;

architecture gate of verification_scope_mismatch is
  signal grants : std_logic_vector(1 downto 0);
begin
  verification : block
  begin
    --@check id=arb.onehot0 scope=arch:rtl grants=grants
  end block verification;
end architecture;
