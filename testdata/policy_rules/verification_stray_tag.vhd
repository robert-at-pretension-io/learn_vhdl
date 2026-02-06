library ieee;
use ieee.std_logic_1164.all;

entity verification_stray_tag is
end entity;

architecture rtl of verification_stray_tag is
  signal grants : std_logic_vector(1 downto 0);
begin
  --@check id=arb.onehot0 scope=arch:rtl grants=grants

  verification : block
  begin
    --@check id=arb.onehot0 scope=arch:rtl grants=grants
  end block verification;
end architecture;
