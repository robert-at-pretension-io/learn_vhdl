library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity clean_power_rules is
  port (
    clk_i    : in std_logic;
    rst_n_i  : in std_logic;
    gate_ok  : in std_logic;
    a_i      : in unsigned(7 downto 0);
    b_i      : in unsigned(7 downto 0);
    q_o      : out unsigned(7 downto 0)
  );
end entity clean_power_rules;

architecture rtl of clean_power_rules is
begin
  seq_p : process(clk_i, rst_n_i)
  begin
    if rst_n_i = '0' then
      q_o <= (others => '0');
    elsif rising_edge(clk_i) then
      if gate_ok = '1' then
        q_o <= a_i * b_i;
      end if;
    end if;
  end process seq_p;
end architecture rtl;
