entity e is
end entity;
architecture a of e is
  signal s : string(1 to 4);
begin
  process
  begin
    s <= "oops";
  end process;
end architecture;
