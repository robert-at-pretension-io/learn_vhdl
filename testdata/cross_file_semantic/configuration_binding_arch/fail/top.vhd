entity child is
end entity;

architecture rtl of child is
begin
end architecture;

architecture gate of child is
begin
end architecture;

entity top is
end entity;

architecture rtl of top is
begin
  u1: entity work.child(gate);
end architecture;

configuration cfg_bad of top is
  for rtl
    for u1: child
      use entity work.child(rtl);
    end for;
  end for;
end configuration;
