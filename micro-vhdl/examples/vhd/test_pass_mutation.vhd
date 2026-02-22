library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity test_pass_mutation is
    port (
        clk : in std_logic;
        rst : in std_logic;
        a   : in unsigned(3 downto 0);
        b   : in unsigned(3 downto 0);
        c   : out unsigned(3 downto 0)
    );
end entity;

architecture rtl of test_pass_mutation is
begin
    process(clk) begin
        if rising_edge(clk) then
            if rst = '1' then
                c <= x"0";
            else
                -- Mutatable operations: ADD, SUB
                c <= a + b;
            end if;
        end if;
    end process;
    
    -- Our verification suite:
    psl assert always (rst = '0' and a = x"2" and b = x"3") |-> next[1](c = x"5");
end architecture;