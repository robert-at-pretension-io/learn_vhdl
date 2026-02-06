library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

package p is
  procedure simAssertion(cond : boolean; msg : string);
end package;

package body p is
  procedure simAssertion(cond : boolean; msg : string) is
  begin
    null;
  end procedure;
end package body;

entity calls_proc_expr_args is
  port(
    clk : in std_logic
  );
end entity;

architecture rtl of calls_proc_expr_args is
  signal a : std_logic_vector(7 downto 0);
  signal b : integer := 0;
begin
  p_proc : process(clk)
  begin
    if rising_edge(clk) then
      simAssertion(a = std_logic_vector(to_unsigned(5, b)), "msg");
    end if;
  end process;
end architecture;
