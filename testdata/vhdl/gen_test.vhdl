architecture rtl of test is
begin
    gen_pipeline : for i in 0 to 3 generate
    begin
        stage_reg : process(clk)
        begin
            if rising_edge(clk) then
                stage_data <= data_in;
            end if;
        end process;
    end generate gen_pipeline;

    guarded_block : block (clk'event and clk = '1') is
    begin
        data_reg <= guarded data_in;
    end block guarded_block;
end architecture;
