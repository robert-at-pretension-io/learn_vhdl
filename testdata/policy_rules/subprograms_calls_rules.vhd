package pkg is
  procedure present_proc;
end package;

package body pkg is
  procedure present_proc is
  begin
    null;
  end procedure;
end package body;

entity subprograms_calls is
end entity;

architecture rtl of subprograms_calls is
begin
  process
    variable tmp : integer;
  begin
    pkg.missing_proc;
    tmp := pkg.missing_func(1);
    wait;
  end process;
end architecture;
