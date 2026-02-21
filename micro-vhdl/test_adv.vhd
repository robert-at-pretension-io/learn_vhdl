library IEEE;
use IEEE.std_logic_1164.all;

entity my_module is
  port (
    clk     : in std_logic;
    idx     : in integer;
    arr_in  : in my_array;
    rec_in  : in my_record;
    out_sig : out std_logic
  );
end entity my_module;

architecture behavior of my_module is
begin
  out_sig <= arr_in(idx) and rec_in.valid;
end architecture behavior;