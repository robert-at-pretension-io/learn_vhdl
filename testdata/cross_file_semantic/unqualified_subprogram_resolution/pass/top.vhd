library work;
use work.pkg.all;

entity top is
end entity;

architecture rtl of top is
  signal y : integer := 0;
begin
  process(all)
  begin
    y <= foo(1);
  end process;
end architecture;
