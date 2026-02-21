library IEEE;
use IEEE.std_logic_1164.all;

entity processor_core is
  generic (
    DATA_WIDTH : integer := 16
  );
  port (
    clk      : in std_logic;
    reset    : in std_logic;
    en       : in std_logic;
    opcode   : in std_logic_vector(1 downto 0);
    operandA : in std_logic_vector(DATA_WIDTH-1 downto 0);
    operandB : in std_logic_vector(DATA_WIDTH-1 downto 0);
    result   : out std_logic_vector(DATA_WIDTH-1 downto 0)
  );
end entity processor_core;

architecture behavior of processor_core is
  signal alu_out  : std_logic_vector(DATA_WIDTH-1 downto 0);
  signal reg_out  : std_logic_vector(DATA_WIDTH-1 downto 0);
  signal is_zero  : std_logic;
begin

  -- Combinational Multiplexing (ALU)
  with opcode select
    alu_out <= operandA + operandB when "00",
               operandA - operandB when "01",
               operandA and operandB when "10",
               operandA xor operandB when others;

  -- Conditional Routing
  is_zero <= '1' when alu_out = 0 else '0';

  -- Sequential Logic (State Machine / Registers)
  process(clk)
  begin
    if rising_edge(clk) then
      if reset = '1' then
        reg_out <= 0;
      elsif en = '1' then
        reg_out <= alu_out;
      else
        reg_out <= reg_out;
      end if;
    end if;
  end process;

  -- Output assignment
  result <= reg_out;

  -- Formal Verification Contract (PSL)
  psl assert always reset |=> is_zero;
  psl assert always stable(clk);

end architecture behavior;