library ieee;
use ieee.std_logic_1164.all;

entity quality_mismatch is
  port (
    buf_p : buffer std_logic
  );
end quality_mismatch;

architecture rtl of quality_mismatch is
  signal dup : std_logic;
  signal DUP : std_logic;
begin
end rtl;
