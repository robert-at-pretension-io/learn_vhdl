library ieee;
use ieee.std_logic_1164.all;

package base_pkg is
  type prot_t is protected
    procedure poke(signal s : out std_logic);
  end protected;
end package;

package body base_pkg is
  type prot_t is protected body
    procedure poke(signal s : out std_logic) is
    begin
      s <= '1';
    end procedure;
  end protected body;
end package body;

package inst_pkg is new work.base_pkg;

entity top is
  port (a : in std_logic);
end entity;

architecture rtl of top is
  component child is
    port (a : in std_logic);
  end component;
  for u1: child use entity work.child(rtl);
begin
  u1: child port map (a => a);
end architecture;
