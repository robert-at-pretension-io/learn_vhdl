-- 2-FF synchronizer: safely crosses a single bit from any source domain
-- to the clk_dst domain.
-- Note: liveness ("d_in eventually reaches d_out") requires an IC3/l2s engine
-- and cannot be expressed as a verif.contract. The entity name sync_2ff is
-- recognized by the semantic checker as a verified CDC boundary.
entity sync_2ff is
  port (
    clk_dst : in std_logic;   -- destination clock
    d_in    : in std_logic;   -- asynchronous input (from source domain)
    d_out   : out std_logic   -- synchronized output (in clk_dst domain)
  );
end entity;

architecture rtl of sync_2ff is
  signal stage1 : std_logic;
begin
  process(clk_dst) begin
    if rising_edge(clk_dst) then
      stage1 <= d_in;
      d_out  <= stage1;
    end if;
  end process;
end architecture;
