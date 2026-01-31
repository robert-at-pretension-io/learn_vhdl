-- Test package body
package test is
    function add(a, b : integer) return integer;
end package test;

package body test is
    function add(a, b : integer) return integer is
    begin
        return a + b;
    end function add;
end package body test;
