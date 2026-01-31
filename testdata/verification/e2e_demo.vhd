library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity verification_e2e is
  port (
    clk   : in std_logic;
    rst   : in std_logic;
    v_out : out std_logic;
    r_in  : in std_logic
  );
end entity;

architecture rtl of verification_e2e is
  type mode_t is (S0, S1);
  signal mode : mode_t;
  signal idx  : unsigned(3 downto 0);
  signal xfer : std_logic;
begin
  verification : block
  begin
    --@check id=fsm.legal_state scope=arch:rtl state=mode
    --@check id=cover.rv.handshake scope=arch:rtl valid=v_out ready=r_in
  end block verification;

  xfer <= v_out and r_in;

  p_fsm : process(clk, rst)
  begin
    if rst = '1' then
      mode <= S0;
    elsif rising_edge(clk) then
      case mode is
        when S0 =>
          mode <= S1;
        when others =>
          mode <= S0;
      end case;
    end if;
  end process;

  p_cnt : process(clk, rst)
  begin
    if rst = '1' then
      idx <= (others => '0');
    elsif rising_edge(clk) then
      idx <= idx + 1;
    end if;
  end process;
end architecture;
