entity Arbiter is
  port (req0, req1 : in std_logic; grant0, grant1 : out std_logic; clk : in std_logic);
  contract
    require: not (req0 = '1' and req1 = '1');  -- mutual exclusion on inputs
    ensure:  not (grant0 = '1' and grant1 = '1');  -- mutual exclusion on outputs
end entity;

architecture rtl of Arbiter is
begin
  process(clk)
  begin
    if rising_edge(clk) then
      grant0 <= req0;
      grant1 <= req1 and not req0;
    end if;
  end process;
end architecture;
