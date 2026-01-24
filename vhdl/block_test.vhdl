architecture rtl of test is
begin
    guarded_block : block (clk'event and clk = '1') is
    begin
        data_reg <= guarded data_in;
    end block guarded_block;
end architecture;
