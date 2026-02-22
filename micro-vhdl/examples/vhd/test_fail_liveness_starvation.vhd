entity arbiter_starve is
  port (req0, req1 : in std_logic; grant0, grant1 : out std_logic; clk : in std_logic);
end entity;

architecture rtl of arbiter_starve is
begin
  process(clk) begin
    if rising_edge(clk) then
      -- Unfair strict priority: req0 always wins if asserted
      if req0 = '1' then
        grant0 <= '1';
        grant1 <= '0';
      elsif req1 = '1' then
        grant0 <= '0';
        grant1 <= '1';
      else
        grant0 <= '0';
        grant1 <= '0';
      end if;
    end if;
  end process;

  -- Liveness property: if req1 occurs, it must EVENTUALLY get a grant
  -- This will fail because an adversarial environment can hold req0 high forever,
  -- starving req1 indefinitely. ABC's l2s will find the lasso (req0='1' loop).
  psl assert always (req1 -> eventually! grant1);
end architecture;
