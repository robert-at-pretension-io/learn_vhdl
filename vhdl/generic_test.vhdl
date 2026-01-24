package test is
    generic (
        type element_type;
        function compare(a, b : element_type) return boolean
    );
end package test;
