architecture rtl of test is
begin
    ready_in <= not fifo_full;
    data_out <= data_reg when valid_out = '1' else (others => 'Z');
    with state select
        status <= "0001" when IDLE,
                  "0010" when START,
                  "0100" when RUN,
                  "1000" when STOP,
                  "1111" when others;
end architecture;
