library work;
use work.pkt_pkg.all;

entity top is
end entity;

architecture rtl of top is
  signal sig : pkt_t;
begin
  u1: entity work.child
    port map (pkt => sig);
end architecture;
