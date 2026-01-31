library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity cdc_demo is
  port (
    clk_a    : in std_logic;
    clk_b    : in std_logic;
    en       : in std_logic;
    async_in : in std_logic_vector(1 downto 0);
    out_sig  : out std_logic_vector(1 downto 0)
  );
end entity;

architecture rtl of cdc_demo is
  signal async2 : std_logic_vector(1 downto 0);
  signal meta   : std_logic_vector(1 downto 0);
  signal sync   : std_logic_vector(1 downto 0);
  signal cnt    : integer := 0;
begin
  p_a : process(clk_a)
  begin
    if rising_edge(clk_a) then
      async2 <= async_in;
    end if;
  end process;

  p_b1 : process(clk_b)
  begin
    if rising_edge(clk_b) then
      meta <= async2;
    end if;
  end process;

  p_b_sync : process(clk_b)
  begin
    if rising_edge(clk_b) then
      sync <= meta;
    end if;
  end process;

  p_b_out : process(clk_b)
  begin
    if rising_edge(clk_b) then
      out_sig <= sync;
      if en = '1' then
        cnt <= cnt * 2;
      end if;
      if cnt = 7 then
        out_sig <= sync;
      end if;
      case cnt is
        when 0 =>
          out_sig <= sync;
        when others =>
          out_sig <= sync;
      end case;
    end if;
  end process;
end architecture;
