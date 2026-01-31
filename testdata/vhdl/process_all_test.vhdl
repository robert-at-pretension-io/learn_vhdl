architecture rtl of test is
begin
    agg_proc : process(all)
        type point is record x, y : integer; end record;
        variable p : point;
    begin
        p := (10, 20);
    end process agg_proc;
end architecture;
