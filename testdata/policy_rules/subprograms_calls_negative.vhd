package pkg is
  function good_func(i : integer) return integer;
  procedure good_proc;
end package;

package body pkg is
  function good_func(i : integer) return integer is
  begin
    return i;
  end function;

  procedure good_proc is
  begin
    null;
  end procedure;
end package body;

entity subprograms_calls_negative is
end entity;

architecture rtl of subprograms_calls_negative is
begin
  process
    variable tmp : integer;
  begin
    tmp := pkg.good_func(1);
    pkg.good_proc;
    wait;
  end process;
end architecture;
