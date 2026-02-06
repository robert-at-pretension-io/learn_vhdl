entity calls_proc_string_commas is
end entity;

architecture rtl of calls_proc_string_commas is
begin
  p0 : process
  begin
    log("A, B, C");
    wait;
  end process;
end architecture;
