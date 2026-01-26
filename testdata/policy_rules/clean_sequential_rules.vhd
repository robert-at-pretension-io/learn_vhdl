library ieee;
use ieee.std_logic_1164.all;

entity clean_sequential_rules is
  port (
    clk_i   : in std_logic;
    rst_n   : in std_logic;
    data_i  : in std_logic;
    data_o  : out std_logic
  );
end entity clean_sequential_rules;

architecture rtl of clean_sequential_rules is
  signal rst_n_sync_meta : std_logic;
  signal rst_n_sync2     : std_logic;
begin
  rst_sync_p : process(clk_i, rst_n)
  begin
    if rst_n = '0' then
      rst_n_sync_meta <= '0';
      rst_n_sync2 <= '0';
    elsif rising_edge(clk_i) then
      rst_n_sync_meta <= rst_n;
      rst_n_sync2 <= rst_n_sync_meta;
    end if;
  end process rst_sync_p;

  seq_p : process(clk_i, rst_n)
  begin
    if rst_n = '0' then
      data_o <= '0';
    elsif rising_edge(clk_i) then
      data_o <= data_i;
    end if;
  end process seq_p;
end architecture rtl;
