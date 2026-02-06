entity top is
end entity;

architecture rtl of top is
begin
  u1: entity work.child(rtl);
end architecture;

configuration cfg_bad of top is
  for rtl
    for u_missing: child
      use entity work.child(rtl);
    end for;
  end for;
end configuration;
