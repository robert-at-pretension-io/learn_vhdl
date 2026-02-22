entity burst_next is
  port (req, clk : in std_logic; valid_out : out std_logic);
end entity;

architecture rtl of burst_next is
  signal active : std_logic;
  signal cnt : std_logic_vector(1 downto 0);
begin
  process(clk) begin
    if rising_edge(clk) then
      if active = '0' then
        if req = '1' then
          active <= '1';
          cnt <= "10"; -- 3 cycles of valid_out! (cnt=2, 1, 0)
          valid_out <= '1';
        else
          valid_out <= '0';
        end if;
      else
        if cnt = "00" then
          active <= '0';
          valid_out <= '0';
        else
          cnt <= cnt - "01";
          valid_out <= '1';
        end if;
      end if;
    end if;
  end process;

  -- Property: a request initiates 4 consecutive cycles of valid_out.
  -- The design only holds it for 3 cycles (cycle 0, 1, 2), so cycle 3 will fail in BMC.
  psl assert always req |-> valid_out;
  psl assert always req |-> next[1](valid_out);
  psl assert always req |-> next[2](valid_out);
  psl assert always req |-> next[3](valid_out);
end architecture;
