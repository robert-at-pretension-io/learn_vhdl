package util_pkg is
  constant WIDTH : integer := 8;
end package util_pkg;

package body util_pkg is
  function add_one(x : integer) return integer is
  begin
    return x + 1;
  end function;
end package body util_pkg;
