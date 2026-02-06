library ieee;
use ieee.std_logic_1164.all;

entity clean_sequential_rules is
  port (
    clk_i   : in std_logic;
    rst_n   : in std_logic;
    data_i  : in std_logic;
    data_o  : out std_logic;
    pipe_o  : out std_logic
  );
end entity clean_sequential_rules;

architecture rtl of clean_sequential_rules is
  signal rst_n_sync_meta : std_logic;
  signal rst_n_sync2     : std_logic;
  signal pipe_r1         : std_logic;
  signal pipe_r2         : std_logic;
begin
  -- Proper 2-stage reset synchronizer (async reset, same edge)
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

  -- Sequential process with async reset (complete sensitivity list)
  seq_p : process(clk_i, rst_n)
  begin
    if rst_n = '0' then
      data_o <= '0';
    elsif rising_edge(clk_i) then
      data_o <= data_i;
    end if;
  end process seq_p;

  -- Pipeline registers: multiple sequential processes on same clock, same edge
  pipe_stage1_p : process(clk_i, rst_n)
  begin
    if rst_n = '0' then
      pipe_r1 <= '0';
    elsif rising_edge(clk_i) then
      pipe_r1 <= data_i;
    end if;
  end process pipe_stage1_p;

  pipe_stage2_p : process(clk_i, rst_n)
  begin
    if rst_n = '0' then
      pipe_r2 <= '0';
      pipe_o <= '0';
    elsif rising_edge(clk_i) then
      pipe_r2 <= pipe_r1;
      pipe_o <= pipe_r2;
    end if;
  end process pipe_stage2_p;
end architecture rtl;
