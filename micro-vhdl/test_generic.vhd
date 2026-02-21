library IEEE;
use IEEE.std_logic_1164.all;

entity child_module is
  generic (
    WIDTH : integer := 8
  );
  port (
    in1 : in std_logic_vector(WIDTH-1 downto 0);
    out1 : out std_logic_vector(WIDTH-1 downto 0)
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
    a : in std_logic_vector(15 downto 0);
    out_sig : out std_logic_vector(15 downto 0)
  );
end entity my_module;

architecture behavior of my_module is
begin
  my_inst : entity work.child_module
    generic map (
      WIDTH => 16
    )
    port map (
      in1 => a,
      out1 => out_sig
    );
end architecture behavior;