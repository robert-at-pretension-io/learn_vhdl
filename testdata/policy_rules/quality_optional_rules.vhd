library ieee;
use ieee.std_logic_1164.all;

package big_pkg is
  signal p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11, p12, p13, p14, p15, p16, p17, p18, p19, p20, p21, p22, p23, p24, p25, p26, p27, p28, p29, p30, p31, p32, p33, p34, p35, p36, p37, p38, p39, p40, p41, p42, p43, p44, p45, p46, p47, p48, p49, p50 : std_logic;
end big_pkg;

entity block1 is
  port (
    a : in std_logic
  );
end block1;

architecture block1 of block1 is
begin
  null;
end block1;

entity mixed_ports is
  port (
    p  : in std_logic;
    q  : out std_logic;
    r  : in std_logic;
    s  : out std_logic;
    t  : in std_logic;
    io : inout std_logic
  );
end mixed_ports;

architecture mixed_ports of mixed_ports is
begin
  null;
end mixed_ports;

entity many_signals_entity is
end many_signals_entity;

architecture many_signals_entity of many_signals_entity is
  signal s0, s1, s2, s3, s4, s5, s6, s7, s8, s9, s10, s11, s12, s13, s14, s15, s16, s17, s18, s19, s20, s21, s22, s23, s24, s25, s26, s27, s28, s29, s30, s31, s32, s33, s34, s35, s36, s37, s38, s39, s40, s41, s42, s43, s44, s45, s46, s47, s48, s49, s50 : std_logic;
begin
  null;
end many_signals_entity;

entity quality_misc is
end quality_misc;

architecture quality_misc of quality_misc is
  signal s : std_logic;
  signal this_is_a_really_long_signal_name_for_quality_checks : std_logic;
  signal magic_width : std_logic_vector(23 downto 0);
begin
  null;
end quality_misc;

entity gen_child is
  generic (
    WIDTH : integer := 8
  );
  port (
    a : in std_logic;
    y : out std_logic
  );
end gen_child;

architecture gen_child of gen_child is
begin
  y <= a;
end gen_child;

entity gen_top is
  port (
    a : in std_logic;
    y : out std_logic
  );
end gen_top;

architecture gen_top of gen_top is
begin
  inst_generic: entity work.gen_child
    generic map (
      WIDTH => 42
    )
    port map (
      a => a,
      y => y
    );
end gen_top;

entity deep_gen is
end deep_gen;

architecture deep_gen of deep_gen is
  signal deep_sig : std_logic;
begin
  gen1: for i in 0 to 0 generate
    gen2: for j in 0 to 0 generate
      gen3: for k in 0 to 0 generate
        gen4: for m in 0 to 0 generate
          gen5: for n in 0 to 0 generate
            deep_sig <= '0';
          end generate;
        end generate;
      end generate;
    end generate;
  end generate;
end deep_gen;
