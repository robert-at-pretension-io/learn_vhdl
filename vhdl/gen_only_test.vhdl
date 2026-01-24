architecture rtl of test is
begin
    gen_pipeline : for i in 0 to 3 generate
    begin
        x <= y;
    end generate gen_pipeline;
end architecture;
