entity invalid_tag_missing_binding is
end entity;

architecture rtl of invalid_tag_missing_binding is
  type state_t is (S_IDLE, S_RUN);
  signal state : state_t;
begin
  verification : block
  begin
    --@check id=fsm.legal_state scope=arch:rtl
  end block verification;
end architecture;
