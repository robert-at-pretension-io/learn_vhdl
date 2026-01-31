library ieee;
use ieee.std_logic_1164.all;

entity signals_rules is
  port (
    in_p  : in std_logic;
    out_p : out std_logic
  );
end signals_rules;

architecture rtl of signals_rules is
  signal dup_sig       : std_logic;
  signal sig_unused    : std_logic;
  signal sig_read_only : std_logic;
  signal sig_multi     : bit;
  signal wide_bus      : std_logic_vector(255 downto 0);
begin
  p_read: process(sig_read_only, in_p)
  begin
    if sig_read_only = '1' then
      out_p <= '1';
    else
      out_p <= in_p;
    end if;
  end process;

  p1: process(in_p)
  begin
    sig_multi <= in_p;
  end process;

  p2: process(in_p)
  begin
    sig_multi <= not in_p;
  end process;

  p_drive_input: process(in_p)
  begin
    in_p <= '0';
  end process;

  p_ghost: process(in_p)
  begin
    ghost_signal <= in_p;
  end process;
end rtl;

entity signals_rules_dup is
  port (
    dummy : in std_logic
  );
end signals_rules_dup;

architecture dup_arch of signals_rules_dup is
  signal dup_sig : std_logic;
begin
  null;
end dup_arch;
