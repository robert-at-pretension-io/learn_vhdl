package pkg is
  function bar(x : integer) return integer;
end package;

package body pkg is
  function bar(x : integer) return integer is
  begin
    return x;
  end function;
end package body;
