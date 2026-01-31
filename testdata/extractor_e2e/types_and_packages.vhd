library ieee;
use ieee.std_logic_1164.all;

package util is
  constant WIDTH : integer := 8;
  type state_t is (IDLE, RUN);
  type rec_t is record
    a : std_logic;
    b : std_logic_vector(WIDTH-1 downto 0);
  end record;
  type arr_t is array (0 to 3) of std_logic;
  subtype idx_t is integer range 0 to 7;
  function inc(x : integer) return integer;
  procedure poke(signal s : out std_logic);
end package;

package body util is
  function inc(x : integer) return integer is
  begin
    return x + 1;
  end function;
  procedure poke(signal s : out std_logic) is
  begin
    s <= '1';
  end procedure;
end package body;
