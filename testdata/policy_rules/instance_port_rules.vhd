-- Positive fixture for Batch 2: instance & port rules
-- Each construct triggers one specific rule.

library ieee;
use ieee.std_logic_1164.all;

entity child_comp is
  generic (
    G_WIDTH : integer  -- generic_without_default: no default value
  );
  port (
    clk    : in  std_logic;
    data_i : in  std_logic;
    data_o : out std_logic;
    aux_o  : out std_logic
  );
end entity child_comp;

architecture rtl of child_comp is
begin
  data_o <= data_i;
  aux_o  <= '0';
end architecture rtl;

entity instance_port_ent is
  port (
    clk   : in  std_logic;
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity instance_port_ent;

architecture rtl of instance_port_ent is
  signal data_sig : std_logic;
  signal aux_sig  : std_ulogic;  -- different type for port_type_mismatch_style
begin

  -- constant_port_connection: literal on output port (data_o => '0')
  -- entity_port_not_connected: aux_o not connected
  -- port_type_mismatch_style: std_ulogic signal on std_logic port (data_i => aux_sig)
  u_child : entity work.child_comp
    generic map (
      G_WIDTH => 8
    )
    port map (
      clk    => clk,
      data_i => aux_sig,
      data_o => '0'
    );

  dout <= data_sig;

end architecture rtl;
