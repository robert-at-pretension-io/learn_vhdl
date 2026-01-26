library ieee;
use ieee.std_logic_1164.all;

entity ports_rules is
  port (
    in_unused      : in std_logic;
    in_used        : in std_logic;
    out_unassigned : out std_logic;
    out_read       : out std_logic;
    io_out         : inout std_logic;
    io_in          : inout std_logic
  );
end ports_rules;

architecture rtl of ports_rules is
begin
  p_io: process(in_used, io_in, out_read)
  begin
    if io_in = '1' then
      io_out <= '1';
    end if;

    if out_read = '1' then
      null;
    end if;
  end process;

  out_read <= in_used;
end rtl;
