library IEEE;
use IEEE.std_logic_1164.all;

entity child_module is
  port (
    in1 : in std_logic;
    out1 : out std_logic
  );
end entity child_module;

architecture behavior of child_module is
begin
  out1 <= not in1;
end architecture behavior;

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
  my_inst : entity work.child_module
    port map (
      in1 => a,
      out1 => temp
    );

  process(clk)
  begin
    if rising_edge(clk) then
      if a = '1' then
        out_sig <= '1';
      elsif b = '1' then
        out_sig <= '0';
      else
        out_sig <= temp;
      end if;
    end if;
  end process;

end architecture behavior;