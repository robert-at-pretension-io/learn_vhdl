library ieee;
use ieee.std_logic_1164.all;

entity core_no_ports is
end core_no_ports;

entity core_without_arch is
  port (
    a : in std_logic
  );
end core_without_arch;

architecture orphan_arch of missing_entity is
begin
end orphan_arch;

entity core_with_inst is
  port (
    clk : in std_logic;
    a   : in std_logic;
    y   : out std_logic
  );
end core_with_inst;

architecture rtl of core_with_inst is
begin
  u_missing: entity work.missing_component
    port map (
      a => a,
      y => y
    );

  comb_case: process(a)
  begin
    case a is
      when '0' => y <= '0';
      when '1' => y <= '1';
    end case;
  end process;
end rtl;
