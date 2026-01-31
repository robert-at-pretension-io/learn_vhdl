library ieee;
use ieee.std_logic_1164.all;

entity read_top is
  port(
    clk : in std_logic;
    en  : in std_logic;
    a   : in std_logic_vector(3 downto 0);
    y   : out std_logic_vector(3 downto 0)
  );
end;

architecture rtl of read_top is
  type rec_t is record
    flag : std_logic;
    data : std_logic_vector(3 downto 0);
  end record;

  signal r : rec_t;
  signal s : std_logic_vector(3 downto 0);
  signal d : std_logic_vector(3 downto 0);
begin
  p_read : process(clk)
  begin
    if rising_edge(clk) then
      r.data <= a;
      s(0) <= a(0);
      if s'length = 4 then
        s <= r.data;
      end if;
      d <= << signal .tb.dut.ext_sig : std_logic_vector(3 downto 0) >>;
    end if;
  end process;

  y <= s after 5 ns;
  y <= s when en = '1' else d;
end;
