entity construct_ready_valid is
  port (
    src_ok  : out std_logic;
    sink_ok : in std_logic
  );
end entity;

architecture rtl of construct_ready_valid is
  signal xfer : std_logic;
begin
  verification : block
  begin
  end block verification;

  xfer <= src_ok and sink_ok;
end architecture;
