architecture rtl of kitchen_sink is
    signal state, next_state : state_type;
    signal counter : unsigned(7 downto 0);
    signal data_reg : std_logic_vector(DATA_WIDTH-1 downto 0);
    signal wr_ptr, rd_ptr : integer range 0 to DEPTH-1;
    signal enable : std_logic := '0';

    attribute mark_debug : string;
    attribute mark_debug of counter : signal is "true";
    attribute keep : boolean;
    attribute keep of data_reg : signal is true;

    constant ZERO : std_logic_vector(DATA_WIDTH-1 downto 0) := (others => '0');

    alias upper_nibble : std_logic_vector(3 downto 0) is data_reg(7 downto 4);

    component adder is
        port (
            a, b : in  std_logic_vector(7 downto 0);
            sum  : out std_logic_vector(8 downto 0)
        );
    end component;

    function parity(vec : std_logic_vector) return std_logic is
        variable result : std_logic := '0';
    begin
        for i in vec'range loop
            result := result xor vec(i);
        end loop;
        return result;
    end function parity;

begin
end architecture;
