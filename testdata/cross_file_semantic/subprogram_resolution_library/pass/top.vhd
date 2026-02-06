library liba;
use liba.pkg.all;

entity top is
end entity;

architecture rtl of top is
  signal y : integer := 0;
begin
  process(all)
  begin
    y <= f(1);
    y <= liba.pkg.f(2);
  end process;
end architecture;
