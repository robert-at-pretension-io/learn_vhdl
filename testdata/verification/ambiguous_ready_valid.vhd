entity ambiguous_ready_valid is
  port (
    a_in : in std_logic;
    b_in : in std_logic
  );
end entity;

architecture rtl of ambiguous_ready_valid is
  signal xfer : std_logic;
begin
  verification : block
  begin
  end block verification;

  xfer <= a_in and b_in;
end architecture;
