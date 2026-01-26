library ieee;
use ieee.std_logic_1164.all;

entity clean_combinational_rules is
  port (
    sel_i : in std_logic;
    a_i   : in std_logic;
    b_i   : in std_logic;
    y_o   : out std_logic
  );
end entity clean_combinational_rules;

architecture rtl of clean_combinational_rules is
  signal y_s : std_logic;
begin
  comb_p : process(sel_i, a_i, b_i)
  begin
    case sel_i is
      when '0' => y_s <= a_i;
      when '1' => y_s <= b_i;
      when others => y_s <= '0';
    end case;
  end process comb_p;

  y_o <= y_s;
end architecture rtl;
