library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity construct_fifo is
  port (
    clk        : in std_logic;
    rst        : in std_logic;
    wr_go      : in std_logic;
    rd_go      : in std_logic;
    full_flag  : out std_logic;
    empty_flag : out std_logic;
    data_in    : in std_logic_vector(7 downto 0);
    data_out   : out std_logic_vector(7 downto 0)
  );
end entity;

architecture rtl of construct_fifo is
  type mem_t is array (0 to 3) of std_logic_vector(7 downto 0);
  signal mem  : mem_t;
  signal wptr : unsigned(1 downto 0);
  signal rptr : unsigned(1 downto 0);
begin
  verification : block
  begin
  end block verification;

  write_p : process(clk, rst)
  begin
    if rst = '1' then
      wptr <= (others => '0');
      full_flag <= '0';
    elsif rising_edge(clk) then
      if wr_go = '1' then
        mem(to_integer(wptr)) <= data_in;
        wptr <= wptr + 1;
      end if;
      full_flag <= '0';
    end if;
  end process;

  read_p : process(clk, rst)
  begin
    if rst = '1' then
      rptr <= (others => '0');
      empty_flag <= '1';
    elsif rising_edge(clk) then
      if rd_go = '1' then
        data_out <= mem(to_integer(rptr));
        rptr <= rptr + 1;
      end if;
      empty_flag <= '0';
    end if;
  end process;
end architecture;
