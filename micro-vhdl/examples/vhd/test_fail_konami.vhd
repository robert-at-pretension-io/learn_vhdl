library ieee;
use ieee.std_logic_1164.all;

entity test_fail_konami is
    port (
        clk   : in std_logic;
        rst   : in std_logic;
        up    : in std_logic;
        down  : in std_logic;
        left  : in std_logic;
        right : in std_logic;
        unlock: out std_logic
    );
end entity;

architecture rtl of test_fail_konami is
    signal state : std_logic_vector(2 downto 0);
begin
    process(clk)
    begin
        if rising_edge(clk) then
            if rst = '1' then
                state <= "000";
                unlock <= '0';
            else
                unlock <= '0';
                
                if state = "000" and up = '1' then
                    state <= "001";
                elsif state = "001" and up = '1' then
                    state <= "010";
                elsif state = "010" and down = '1' then
                    state <= "011";
                elsif state = "011" and down = '1' then
                    state <= "100";
                elsif state = "100" and left = '1' then
                    state <= "101";
                elsif state = "101" and right = '1' then
                    state <= "110";
                    unlock <= '1';
                elsif (up = '1' or down = '1' or left = '1' or right = '1') then
                    state <= "000";
                end if;
            end if;
        end if;
    end process;
    
    -- Formal intent: The system should NEVER be unlocked.
    -- (This will fail. Z3/BTORMC will discover the exact "Konami code" sequence 
    -- required to unlock the system, and the VCD trace will perfectly visualize it).
    psl assert always (unlock = '0');
end architecture;