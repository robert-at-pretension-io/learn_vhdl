architecture rtl of test is
begin
    gen_reset : if USE_RESET generate
        x <= y;
    end generate gen_reset;
end architecture;
