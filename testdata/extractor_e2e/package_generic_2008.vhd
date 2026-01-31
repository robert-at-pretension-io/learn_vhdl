package gen_pkg is
  generic (
    type T;
    constant WIDTH : integer := 4
  );
  type arr_t is array (0 to WIDTH-1) of T;
end package;
