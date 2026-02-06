library ieee;
use ieee.std_logic_1164.all;

entity rec_arr_tb is
end entity;

architecture rtl of rec_arr_tb is
  type rec_t is record
    field : std_logic_vector(7 downto 0);
  end record;
  type rec_arr_t is array (natural range <>) of rec_t;
  signal recs : rec_arr_t(0 to 1);
  signal outv : std_logic_vector(7 downto 0);
begin
  p_rec : process
  begin
    outv <= recs(1).field(3 downto 0) & recs(0).field(7 downto 4);
    wait;
  end process;
end architecture;
