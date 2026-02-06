package pkg is
  function f(x : integer) return integer;
end package;

package body pkg is
  function f(x : integer) return integer is
  begin
    return x + 1;
  end;
end package body;
