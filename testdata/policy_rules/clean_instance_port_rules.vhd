-- Negative fixture for Batch 2: instance & port rules
-- Clean code that should NOT trigger any of the new rules.

library ieee;
use ieee.std_logic_1164.all;

entity clean_child is
  generic (
    G_WIDTH : integer := 8  -- has default
  );
  port (
    clk    : in  std_logic;
    data_i : in  std_logic;
    data_o : out std_logic
  );
end entity clean_child;

architecture rtl of clean_child is
begin
  data_o <= data_i;
end architecture rtl;

entity clean_instance_port_ent is
  port (
    clk   : in  std_logic;
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity clean_instance_port_ent;

architecture rtl of clean_instance_port_ent is
  signal data_sig : std_logic;
begin

  -- All ports connected, signal types match, no literal on output
  u_child : entity work.clean_child
    generic map (
      G_WIDTH => 8
    )
    port map (
      clk    => clk,
      data_i => din,
      data_o => data_sig
    );

  dout <= data_sig;

end architecture rtl;
