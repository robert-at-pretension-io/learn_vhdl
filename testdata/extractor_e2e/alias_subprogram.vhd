library ieee;
use ieee.std_logic_1164.all;

package alias_pkg is
  type int_vec is array (natural range <>) of integer;
  function to_std_logic_vector(iv : int_vec) return std_logic_vector;
  alias to_slv is to_std_logic_vector[int_vec return std_logic_vector];
end package;
