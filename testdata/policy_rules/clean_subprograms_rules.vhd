library ieee;
use ieee.std_logic_1164.all;

entity clean_subprograms_rules is
  port (
    data_i : in std_logic;
    data_o : out std_logic
  );
end entity clean_subprograms_rules;

architecture rtl of clean_subprograms_rules is
  function pass_through(x : in std_logic) return std_logic is
  begin
    return x;
  end function pass_through;

  procedure copy_sig(signal src : in std_logic; signal dst : out std_logic) is
  begin
    dst <= src;
  end procedure copy_sig;

  signal data_s : std_logic;
begin
  data_s <= pass_through(data_i);
  copy_sig(data_s, data_o);
end architecture rtl;
