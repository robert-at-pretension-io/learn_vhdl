package pkg is
  function foo(x : integer) return integer;
end package;

package body pkg is
  function foo(x : integer) return integer is
  begin
    return x + 1;
  end;
end package body;
