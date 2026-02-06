library ieee;
use ieee.std_logic_1164.all;

entity clean_combinational_rules is
  port (
    sel_i  : in std_logic;
    sel2_i : in std_logic_vector(1 downto 0);
    a_i    : in std_logic;
    b_i    : in std_logic;
    c_i    : in std_logic;
    d_i    : in std_logic;
    y_o    : out std_logic;
    z_o    : out std_logic;
    w_o    : out std_logic
  );
end entity clean_combinational_rules;

architecture rtl of clean_combinational_rules is
  signal y_s : std_logic;
  signal z_s : std_logic;
  signal w_s : std_logic;
begin
  -- Simple case with when others (no latch)
  comb_p : process(sel_i, a_i, b_i)
  begin
    case sel_i is
      when '0' => y_s <= a_i;
      when '1' => y_s <= b_i;
      when others => y_s <= '0';
    end case;
  end process comb_p;

  -- Complex if/elsif/else with complete sensitivity list (no latch)
  mux_p : process(sel2_i, a_i, b_i, c_i, d_i)
  begin
    if sel2_i = "00" then
      z_s <= a_i;
    elsif sel2_i = "01" then
      z_s <= b_i;
    elsif sel2_i = "10" then
      z_s <= c_i;
    else
      z_s <= d_i;
    end if;
  end process mux_p;

  -- Default-value style: default assignment prevents latch on conditional path
  default_p : process(sel_i, a_i)
  begin
    w_s <= '0';
    if sel_i = '1' then
      w_s <= a_i;
    end if;
  end process default_p;

  y_o <= y_s;
  z_o <= z_s;
  w_o <= w_s;
end architecture rtl;
