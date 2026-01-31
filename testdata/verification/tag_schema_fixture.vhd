entity tag_schema_fixture is
end entity;

architecture rtl of tag_schema_fixture is
  signal state : integer;
begin
  verification : block
  begin
    --@check id=fsm.legal_state scope=arch:rtl state=state
    --@check id=fsm.reset_known scope=bad_scope
  end block verification;
end architecture;
