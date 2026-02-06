entity child is
end entity;

architecture rtl of child is
begin
end architecture;

configuration cfg_bad of child is
  for missing_arch
  end for;
end configuration;
