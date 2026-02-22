entity Dummy is
  port (clk : in std_logic);
end entity;

architecture rtl of Dummy is
begin
end architecture;

verification CheckMult is
  symbolic a, b : std_logic_vector(31 downto 0);
  psl assert always (a * b = b * a);
end verification;
