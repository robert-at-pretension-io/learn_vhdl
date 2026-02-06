library ieee;
use ieee.std_logic_1164.all;

entity alias_record_calls is
  port(
    clk : in std_logic
  );
end entity;

architecture rtl of alias_record_calls is
  type resp_t is record
    User : std_logic_vector(3 downto 0);
    Valid : std_logic;
  end record;
  signal Resp : resp_t;
begin
  p_alias : process(clk)
    alias WR : resp_t is Resp;
  begin
    if rising_edge(clk) then
      WR.User <= (others => '0');
      WR.Valid <= '1';
    end if;
  end process;
end architecture;
