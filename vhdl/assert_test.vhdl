architecture rtl of test is
begin
    p : process(clk)
    begin
        assert counter < 256
            report "Overflow at " & time'image(now)
            severity error;
    end process p;
end architecture;
