library ieee;
use ieee.std_logic_1164.all;

entity top is
  port (
    clk : in std_logic;
    sel : in std_logic_vector(1 downto 0);
    din : in std_logic_vector(3 downto 0);
    dout : out std_logic_vector(3 downto 0)
  );
end entity;

architecture rtl of top is
  component child is
    port (
      clk : in std_logic;
      d   : in std_logic;
      q   : out std_logic
    );
  end component;

  signal a, b, c : std_logic;
  signal vec : std_logic_vector(3 downto 0);
begin
  -- simple concurrent assignment
  a <= b;
  -- conditional concurrent assignment
  c <= a when sel(0) = '1' else b;
  -- selected concurrent assignment
  with sel select vec <= "0000" when "00", "1111" when others;

  u1 : entity work.child
    port map (clk => clk, d => a, q => b);

  gen_for : for i in 0 to 1 generate
    signal gsig : std_logic;
  begin
    gsig <= a;
    u2 : entity work.child port map (clk => clk, d => gsig, q => c);
  end generate;

  gen_if : if a = '1' generate
    signal ifsig : std_logic;
  begin
    ifsig <= b;
  end generate;

  gen_case : case sel generate
    when "00" =>
      c <= a;
    when others =>
      c <= b;
  end generate;
end architecture;

configuration cfg_top of top is
end configuration;
