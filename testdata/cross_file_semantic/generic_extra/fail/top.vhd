entity top is
end entity;

architecture rtl of top is
begin
  u1: entity work.child
    generic map (G_EXTRA => 1);
end architecture;
