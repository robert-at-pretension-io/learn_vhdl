entity missing_cover_companion is
end entity;

architecture rtl of missing_cover_companion is
  signal v_sig : std_logic;
  signal r_sig : std_logic;
begin
  verification : block
  begin
    --@check id=rv.stable_while_stalled scope=arch:rtl valid=v_sig ready=r_sig
  end block verification;
end architecture;
