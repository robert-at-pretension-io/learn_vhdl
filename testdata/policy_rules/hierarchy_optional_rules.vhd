library ieee;
use ieee.std_logic_1164.all;

entity leaf is
  port (
    a : in std_logic;
    b : in std_logic;
    y : out std_logic
  );
end leaf;

architecture rtl of leaf is
begin
  y <= a and b;
end rtl;

entity hier_top is
  port (
    a : in std_logic;
    b : in std_logic;
    y : out std_logic
  );
end hier_top;

architecture rtl of hier_top is
  signal sig_a : std_logic;
  signal sig_b : std_logic;
  signal sig_y : std_logic;
begin
  sig_a <= a;
  sig_b <= b;

  leaf: entity work.leaf
    port map (
      a => sig_a,
      b => sig_b,
      y => sig_y
    );

  badinst: entity work.leaf
    port map (
      a => '1',
      b => sig_b,
      y => open
    );

  sparse_inst: entity work.leaf
    port map (
      a => sig_a
    );

  u0: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u1: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u2: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u3: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u4: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u5: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u6: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u7: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u8: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u9: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u10: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u11: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u12: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u13: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u14: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u15: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u16: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u17: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u18: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u19: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  u20: entity work.leaf
    port map (a => sig_a, b => sig_b, y => sig_y);

  y <= sig_y;
end rtl;
