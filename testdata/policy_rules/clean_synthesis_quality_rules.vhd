-- Negative fixture for Batch 3: synthesis & quality rules
-- Clean code that should NOT trigger any of the new rules.

library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity clean_synthesis_quality_ent is
  port (
    clk   : in  std_logic;
    rst_n : in  std_logic;
    sel   : in  std_logic_vector(1 downto 0);
    din   : in  std_logic;
    dout  : out std_logic
  );
end entity clean_synthesis_quality_ent;

architecture rtl of clean_synthesis_quality_ent is
  signal data_reg : std_logic;
begin

  -- Good case: has explicit alternatives plus others
  good_case_proc : process(sel, din)
  begin
    case sel is
      when "00" =>
        dout <= din;
      when "01" =>
        dout <= not din;
      when others =>
        dout <= '0';
    end case;
  end process good_case_proc;

  -- Good sequential: has reset
  good_seq_proc : process(clk, rst_n)
  begin
    if rst_n = '0' then
      data_reg <= '0';
    elsif rising_edge(clk) then
      data_reg <= din;
    end if;
  end process good_seq_proc;

  -- Good function: has body
  function parity(d : std_logic_vector) return std_logic is
    variable result : std_logic := '0';
  begin
    for i in d'range loop
      result := result xor d(i);
    end loop;
    return result;
  end function parity;

end architecture rtl;
