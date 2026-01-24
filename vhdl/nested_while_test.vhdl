architecture rtl of test is
begin
    loop_proc : process(clk)
        variable i : integer;
    begin
        if rising_edge(clk) then
            for i in 0 to 7 loop
                shift_reg(i+1) <= shift_reg(i);
            end loop;

            i := 0;
            while i < 8 loop
                data(i) <= '0';
                i := i + 1;
            end loop;
        end if;
    end process loop_proc;
end architecture;
