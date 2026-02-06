-- Positive fixture for variable analysis rules.

library ieee;
use ieee.std_logic_1164.all;

entity variable_rules_ent is
  port (
    clk   : in  std_logic;
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity variable_rules_ent;

architecture rtl of variable_rules_ent is
  signal data_reg : std_logic;
begin

  -- unused_variable: dead_var is never referenced
  -- variable_shadows_signal: data_reg shadows the architecture signal
  -- uninitialized_variable_read: uninit_v is read but never assigned
  var_proc : process(clk)
    variable dead_var  : integer;
    variable data_reg  : std_logic;
    variable uninit_v  : std_logic;
  begin
    if rising_edge(clk) then
      dout <= uninit_v;
    end if;
  end process;

end architecture rtl;
