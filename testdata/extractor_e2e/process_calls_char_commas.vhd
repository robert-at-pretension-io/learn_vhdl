entity calls_proc_char_commas is
end entity;

architecture rtl of calls_proc_char_commas is
begin
  p0 : process
  begin
    log_char(',', "msg");
    wait;
  end process;
end architecture;
