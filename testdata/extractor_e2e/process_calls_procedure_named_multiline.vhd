library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

package p is
  procedure DoAxiValidHandshake(
    signal Clk : in std_logic;
    signal Valid : out std_logic;
    signal Ready : in std_logic;
    constant tpd_Clk_Valid : in time;
    constant AlertLogID : in integer;
    constant TimeOutMessage : in string;
    constant TimeOutPeriod : in time
  );
end package;

package body p is
  procedure DoAxiValidHandshake(
    signal Clk : in std_logic;
    signal Valid : out std_logic;
    signal Ready : in std_logic;
    constant tpd_Clk_Valid : in time;
    constant AlertLogID : in integer;
    constant TimeOutMessage : in string;
    constant TimeOutPeriod : in time
  ) is
  begin
    null;
  end procedure;
end package body;

entity calls_proc_named_multiline is
  port(
    clk : in std_logic
  );
end entity;

architecture rtl of calls_proc_named_multiline is
  signal valid_s : std_logic := '0';
  signal ready_s : std_logic := '0';
  constant tpd_Clk : time := 1 ns;
  constant alert_id : integer := 0;
begin
  p_multi : process(clk)
  begin
    DoAxiValidHandshake (
      Clk            => clk,
      Valid          => valid_s,
      Ready          => ready_s,
      tpd_Clk_Valid  => tpd_Clk,
      AlertLogID     => alert_id,
      TimeOutMessage => "msg" & to_string(1),
      TimeOutPeriod  => 2 * tpd_Clk
    );
    wait;
  end process;
end architecture;
