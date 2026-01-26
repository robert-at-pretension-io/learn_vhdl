library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity power_rules is
  port (
    clk    : in std_logic;
    enable : in std_logic;
    a      : in integer;
    b      : in integer;
    q      : out integer
  );
end power_rules;

architecture rtl of power_rules is
  signal mult_q : integer;
  signal exp_q : integer;
  signal guard_q : integer;
  signal comb_mult : integer;
  signal wide_a : unsigned(31 downto 0);
  signal wide_b : unsigned(31 downto 0);
  signal wide_q : unsigned(63 downto 0);
  signal reg0 : std_logic;
  signal reg1 : std_logic;
  signal reg2 : std_logic;
  signal reg3 : std_logic;
  signal reg4 : std_logic;
  signal reg5 : std_logic;
begin
  p_combo: process(a, b)
  begin
    mult_q <= a * b;
    exp_q <= a ** 2;
    q <= a / b;
    comb_mult <= a * b;
  end process;

  p_guard: process(a, b, enable)
  begin
    if enable = '1' then
      guard_q <= a * b;
    end if;
  end process;

  p_wide: process(wide_a, wide_b)
  begin
    wide_q <= wide_a * wide_b;
  end process;

  p_regs: process(clk)
  begin
    if rising_edge(clk) then
      if enable = '1' then
        reg0 <= '1';
        reg1 <= '1';
        reg2 <= '1';
        reg3 <= '1';
        reg4 <= '1';
        reg5 <= '1';
      end if;
    end if;
  end process;
end rtl;
