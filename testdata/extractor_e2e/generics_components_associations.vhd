library ieee, work;
use ieee.std_logic_1164.all, work.util_pkg.all;
context work.my_ctx;

entity gen_top is
  generic (
    WIDTH : integer := 8;
    MODE  : string := "fast"
  );
  port(
    clk  : in std_logic;
    din  : in std_logic_vector(WIDTH-1 downto 0);
    dout : out std_logic_vector(WIDTH-1 downto 0)
  );
end;

architecture rtl of gen_top is
  component child is
    generic (
      G_WIDTH : integer := 4
    );
    port(
      clk : in std_logic;
      d   : in std_logic_vector(G_WIDTH-1 downto 0);
      q   : out std_logic_vector(G_WIDTH-1 downto 0)
    );
  end component;

  signal s : std_logic_vector(WIDTH-1 downto 0);
begin
  u_child : entity work.child
    generic map (G_WIDTH => WIDTH)
    port map (clk => clk, d => din, q => s);

  u_child_pos : entity work.child
    generic map (WIDTH)
    port map (clk, din, open);

  dout <= s;
end;
