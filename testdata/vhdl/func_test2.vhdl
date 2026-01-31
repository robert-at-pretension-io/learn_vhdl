function parity(vec : std_logic_vector) return std_logic is
    variable result : std_logic := '0';
begin
    for i in vec'range loop
        result := result xor vec(i);
    end loop;
    return result;
end function parity;
