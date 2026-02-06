package pkg is
  function foo(x : integer) return integer;
  function foo(x : natural) return integer;
end package;

package body pkg is
  function foo(x : integer) return integer is
  begin
    return x;
  end function;

  function foo(x : natural) return integer is
  begin
    return integer(x);
  end function;
end package body;
