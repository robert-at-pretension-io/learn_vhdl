library ieee;
use ieee.std_logic_1164.all;

entity child_comp is
  port (
    clk_i : in std_logic;
    a_i   : in std_logic;
    b_i   : in std_logic;
    y_o   : out std_logic
  );
end entity child_comp;

architecture rtl of child_comp is
begin
  y_o <= a_i and b_i;
end architecture rtl;

entity clean_instances_rules is
  port (
    clk_i : in std_logic;
    a_i   : in std_logic;
    b_i   : in std_logic;
    y_o   : out std_logic
  );
end entity clean_instances_rules;

architecture rtl of clean_instances_rules is
  signal a_s : std_logic;
  signal b_s : std_logic;
  signal y_s : std_logic;
begin
  a_s <= a_i;
  b_s <= b_i;

  u_child : entity work.child_comp
    port map (
      clk_i => clk_i,
      a_i   => a_s,
      b_i   => b_s,
      y_o   => y_s
    );

  y_o <= y_s;
end architecture rtl;
