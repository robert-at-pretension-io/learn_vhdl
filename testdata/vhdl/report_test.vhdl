architecture rtl of test is
begin
    p : process(clk)
    begin
        if valid_out = '1' and ready_out = '1' then
            report "Output: " & to_hstring(data_out);
        end if;
    end process p;
end architecture;
