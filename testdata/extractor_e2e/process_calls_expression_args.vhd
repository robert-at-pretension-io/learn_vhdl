library ieee;
use ieee.std_logic_1164.all;

package math_pkg is
  function int2vec(x : integer; width : integer) return std_logic_vector;
end package;

package body math_pkg is
  function int2vec(x : integer; width : integer) return std_logic_vector is
  begin
    return (others => '0');
  end function;
end package body;

entity calls_expression_args is
  port(
    clk : in std_logic
  );
end entity;

architecture rtl of calls_expression_args is
  signal a : integer := 0;
  signal b : integer := 0;
  signal y : std_logic_vector(7 downto 0);
begin
  p_expr : process(clk)
  begin
    if rising_edge(clk) then
      y <= math_pkg.int2vec((a * b), 8);
    end if;
  end process;
end architecture;
