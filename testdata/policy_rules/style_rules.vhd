library ieee;
use ieee.std_logic_1164.all;
use ieee.std_logic_unsigned.all;

entity big_entity is
  port (
    p0 : in std_logic;
    p1 : in std_logic;
    p2 : in std_logic;
    p3 : in std_logic;
    p4 : in std_logic;
    p5 : in std_logic;
    p6 : in std_logic;
    p7 : in std_logic;
    p8 : in std_logic;
    p9 : in std_logic;
    p10 : in std_logic;
    p11 : in std_logic;
    p12 : in std_logic;
    p13 : in std_logic;
    p14 : in std_logic;
    p15 : in std_logic;
    p16 : in std_logic;
    p17 : in std_logic;
    p18 : in std_logic;
    p19 : in std_logic;
    p20 : in std_logic;
    p21 : in std_logic;
    p22 : in std_logic;
    p23 : in std_logic;
    p24 : in std_logic;
    p25 : in std_logic;
    p26 : in std_logic;
    p27 : in std_logic;
    p28 : in std_logic;
    p29 : in std_logic;
    p30 : in std_logic;
    p31 : in std_logic;
    p32 : in std_logic;
    p33 : in std_logic;
    p34 : in std_logic;
    p35 : in std_logic;
    p36 : in std_logic;
    p37 : in std_logic;
    p38 : in std_logic;
    p39 : in std_logic;
    p40 : in std_logic;
    p41 : in std_logic;
    p42 : in std_logic;
    p43 : in std_logic;
    p44 : in std_logic;
    p45 : in std_logic;
    p46 : in std_logic;
    p47 : in std_logic;
    p48 : in std_logic;
    p49 : in std_logic;
    p50 : in std_logic
  );
end big_entity;

architecture core of big_entity is
  signal tmp : std_logic;
begin
  process(p0)
  begin
    tmp <= p0;
  end process;
end core;

entity small_entity is
  port (
    a : in std_logic
  );
end small_entity;

architecture rtl of small_entity is
begin
  null;
end rtl;
