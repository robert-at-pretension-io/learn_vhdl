entity clean_configurations_rules is
  port (
    data_i : in bit;
    data_o : out bit
  );
end entity clean_configurations_rules;

architecture rtl of clean_configurations_rules is
begin
  data_o <= data_i;
end architecture rtl;

configuration cfg_clean of clean_configurations_rules is
end configuration cfg_clean;
