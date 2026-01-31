context ieee.std_logic_1164;
library ieee;
use ieee.std_logic_1164.all;

entity psl_demo is
  port (
    clk : in std_logic;
    a   : in std_logic;
    b   : in std_logic;
    z   : out std_logic
  );
end entity;

architecture rtl of psl_demo is
  signal v : std_logic;
begin
  default clock is rising_edge(clk);
  property p1 is always a;
  sequence s1 is {a; b};
  assert always a;
  cover {a; b};
  assume a;
  restrict {a; b};

  p_wait : process
  begin
    wait until clk = '1';
    v <= a;
  end process;

  p_all : process(all)
  begin
    z <= v;
  end process;
end architecture;
