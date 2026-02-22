library ieee;
use ieee.std_logic_1164.all;

entity test_fail_counter is
    port (
        clk : in std_logic;
        rst : in std_logic;
        req : in std_logic;
        cnt_out : out std_logic_vector(3 downto 0)
    );
end entity;

architecture rtl of test_fail_counter is
    signal cnt : std_logic_vector(3 downto 0);
begin
    process(clk)
    begin
        if rising_edge(clk) then
            if rst = '1' then
                cnt <= x"0";
            elsif req = '1' then
                cnt <= cnt + 1;
            end if;
        end if;
    end process;
    
    cnt_out <= cnt;
    
    -- Assert that cnt never reaches 3 (will fail on cycle 3)
    psl assert always (cnt /= x"3");
end architecture;