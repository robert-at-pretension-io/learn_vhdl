library IEEE;
use IEEE.std_logic_1164.all;

entity timer is
  port (
    clk    : in std_logic;
    reset  : in std_logic;
    en     : in std_logic;
    limit  : in std_logic_vector(7 downto 0);
    done   : out std_logic
  );
end entity timer;

architecture behavior of timer is
  signal count : std_logic_vector(7 downto 0);
  signal next_count : std_logic_vector(7 downto 0);
  signal is_done : std_logic;
begin
  is_done <= '1' when count = limit else '0';
  done <= is_done;

  next_count <= count + 1 when is_done = '0' else 0;

  process(clk)
  begin
    if rising_edge(clk) then
      if reset = '1' then
        count <= 0;
      elsif en = '1' then
        count <= next_count;
      else
        count <= count;
      end if;
    end if;
  end process;

end architecture behavior;