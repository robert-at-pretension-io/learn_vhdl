function foo return integer is
begin
    for i in vec'range loop
        x := x + 1;
    end loop;
end;
