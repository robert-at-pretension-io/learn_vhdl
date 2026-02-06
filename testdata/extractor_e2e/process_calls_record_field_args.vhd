library ieee;
use ieee.std_logic_1164.all;

entity call_record_arg is
end entity;

architecture rtl of call_record_arg is
  type rec_t is record
    Field : std_logic_vector(3 downto 0);
  end record;
  signal R : rec_t;
begin
  p : process
    variable tmp : std_logic_vector(3 downto 0);
  begin
    tmp := Foo(R.Field);
    tmp := Foo(R.Field'length);
    wait;
  end process;
end architecture;
