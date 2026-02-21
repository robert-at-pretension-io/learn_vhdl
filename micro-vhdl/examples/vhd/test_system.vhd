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
begin
  result <= operandA;
end architecture behavior;

entity system_top is
  port (
    sys_clk   : in std_logic;
    sys_rst   : in std_logic;
    sys_en    : in std_logic;
    opA_0     : in std_logic_vector(31 downto 0);
    opB_0     : in std_logic_vector(31 downto 0);
    out_0     : out std_logic_vector(31 downto 0)
  );
end entity system_top;

architecture behavior of system_top is
begin

  core_0 : entity work.processor_core
    generic map (
      DATA_WIDTH => 32
    )
    port map (
      clk      => sys_clk,
      reset    => sys_rst,
      en       => sys_en,
      opcode   => "00",
      operandA => opA_0,
      operandB => opB_0,
      result   => out_0
    );

end architecture behavior;