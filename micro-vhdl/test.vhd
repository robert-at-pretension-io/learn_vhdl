library IEEE;
use IEEE.std_logic_1164.all;

entity my_module is
  port (
    clk : in std_logic;
    a, b : in std_logic;
    out_sig : out std_logic
  );
end entity my_module;

architecture behavior of my_module is
  signal temp : std_logic;
begin
  temp <= a and b;

  process(clk)
  begin
    if rising_edge(clk) then
      out_sig <= temp;
    end if;
  end process;

  my_inst : entity work.child_module
    port map (
      in1 => a,
      out1 => temp
    );

end architecture behavior;