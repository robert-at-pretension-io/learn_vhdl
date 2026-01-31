library ieee;
use ieee.std_logic_1164.all;

entity edge_top is
  port (
    clk : in std_logic;
    a   : in std_logic;
    b   : in std_logic;
    y   : out std_logic
  );
end entity;

architecture rtl of edge_top is
  component child is
    port (a : in std_logic; y : out std_logic);
  end component;

  signal s : std_logic;
  signal t : std_logic;
  disconnect all : std_logic after 5 ns;
begin
  property p1 is next[2](a);
  assert next[2](a);

  p_case : process(clk)
  begin
    if rising_edge(clk) then
      case? s is
        when '0' => t <= a;
        when others => t <= b;
      end case?;
    end if;
  end process;

  gen_for : for i in 0 to 1 generate
  begin
    u1 : entity work.child port map (a => a, y => y);
  end generate;
end architecture;

configuration cfg_edge of edge_top is
  for rtl
    for gen_for(0)
      for u1: child
        use entity work.child(rtl);
      end for;
    end for;
  end for;
end configuration;
