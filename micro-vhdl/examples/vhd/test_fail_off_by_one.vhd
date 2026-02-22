library ieee;
use ieee.std_logic_1164.all;

entity test_fail_off_by_one is
    port (
        clk   : in std_logic;
        rst   : in std_logic;
        start : in std_logic;
        done  : out std_logic
    );
end entity;

architecture rtl of test_fail_off_by_one is
    signal delay1 : std_logic;
    signal delay2 : std_logic;
begin
    process(clk)
    begin
        if rising_edge(clk) then
            if rst = '1' then
                delay1 <= '0';
                delay2 <= '0';
                done   <= '0';
            else
                -- Shift register to delay the start signal
                delay1 <= start;
                delay2 <= delay1;
                
                -- BUG: Should be 'done <= delay2' for a strict 3-cycle delay
                -- but we accidentally wire it to delay1 (2-cycle delay) under a specific condition!
                if start = '1' and delay1 = '1' then
                    -- Back to back starts trigger the bug
                    done <= delay1;
                else
                    done <= delay2;
                end if;
            end if;
        end if;
    end process;

    -- Formal Intent: The specification requires a strict 3-cycle processing delay.
    -- If 'start' is asserted, 'done' MUST be asserted exactly 3 cycles later.
    -- (This will fail on back-to-back requests, and the VCD trace will perfectly visualize the overlapped signals).
    psl assert always start = '1' |-> next[3](done = '1');
end architecture;