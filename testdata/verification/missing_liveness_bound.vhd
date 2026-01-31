entity missing_liveness_bound is
end entity;

architecture rtl of missing_liveness_bound is
  signal v_sig : std_logic;
  signal r_sig : std_logic;
begin
  verification : block
  begin
    --@check id=rv.eventual_progress_bounded scope=arch:rtl valid=v_sig ready=r_sig
  end block verification;
end architecture;
