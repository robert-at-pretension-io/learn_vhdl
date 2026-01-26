library ieee;
use ieee.std_logic_1164.all;

entity combinational_rules is
end combinational_rules;

architecture rtl of combinational_rules is
  signal a : std_logic;
  signal b : std_logic;
  signal c : std_logic;
  signal r0 : std_logic;
  signal r1 : std_logic;
  signal r2 : std_logic;
  signal r3 : std_logic;
  signal r4 : std_logic;
  signal r5 : std_logic;
  signal r6 : std_logic;
  signal r7 : std_logic;
  signal r8 : std_logic;
  signal r9 : std_logic;
  signal r10 : std_logic;
  signal r11 : std_logic;
  signal r12 : std_logic;
  signal r13 : std_logic;
  signal r14 : std_logic;
  signal r15 : std_logic;
  signal w0 : std_logic;
  signal w1 : std_logic;
  signal w2 : std_logic;
  signal w3 : std_logic;
  signal w4 : std_logic;
  signal w5 : std_logic;
  signal w6 : std_logic;
  signal w7 : std_logic;
  signal w8 : std_logic;
  signal w9 : std_logic;
  signal w10 : std_logic;
  signal w11 : std_logic;
  signal w12 : std_logic;
  signal w13 : std_logic;
  signal w14 : std_logic;
  signal w15 : std_logic;
begin
  p_empty: process
  begin
    a <= a;
  end process;

  p_ab: process(a, b)
  begin
    b <= a;
  end process;

  p_ba: process(a, b)
  begin
    a <= b;
  end process;

  p_cycle: process(a, b, c)
  begin
    c <= b;
    a <= c;
  end process;

  p_all: process(all)
  begin
    b <= a;
  end process;

  p_large: process(r0, r1, r2, r3, r4, r5, r6, r7, r8, r9, r10, r11, r12, r13, r14, r15)
  begin
    w0 <= r0;
    w1 <= r1;
    w2 <= r2;
    w3 <= r3;
    w4 <= r4;
    w5 <= r5;
    w6 <= r6;
    w7 <= r7;
    w8 <= r8;
    w9 <= r9;
    w10 <= r10;
    w11 <= r11;
    w12 <= r12;
    w13 <= r13;
    w14 <= r14;
    w15 <= r15;
  end process;
end rtl;
