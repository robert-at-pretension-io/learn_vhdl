entity Arbiter is
  port (req0, req1 : in std_logic; grant0, grant1 : out std_logic; clk : in std_logic);
  contract
    require: not (req0 = '1' and req1 = '1');
    ensure:  not (grant0 = '1' and grant1 = '1');
end entity;

architecture rtl of Arbiter is
begin
  grant0 <= req0;
  grant1 <= req1;
end architecture;
