-- ============================================================================
-- VHDL-2008 Kitchen Sink Test File
-- Covers a wide range of language constructs for grammar testing
-- ============================================================================

-- =========================
-- Library and Use Clauses
-- =========================
library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;
use ieee.math_real.all;

library work;
use work.my_package.all;
use work.other_package.some_function;

-- =========================
-- Package Declaration
-- =========================
package my_package is
    -- Constants
    constant DATA_WIDTH : integer := 8;
    constant CLK_PERIOD : time := 10 ns;
    constant PI_APPROX : real := 3.14159;
    constant INIT_VEC : std_logic_vector(7 downto 0) := x"AA";
    constant BIN_VAL : std_logic_vector(3 downto 0) := b"1010";
    constant OCT_VAL : std_logic_vector(11 downto 0) := o"7654";
    
    -- Type declarations
    type state_type is (IDLE, START, RUN, STOP, ERROR);
    type int_array is array (natural range <>) of integer;
    type matrix is array (0 to 3, 0 to 3) of std_logic;
    type mem_type is array (0 to 255) of std_logic_vector(7 downto 0);
    
    -- Subtypes
    subtype byte is std_logic_vector(7 downto 0);
    subtype nibble is std_logic_vector(3 downto 0);
    subtype small_int is integer range 0 to 100;
    
    -- Record type
    type data_record is record
        valid : std_logic;
        data  : std_logic_vector(31 downto 0);
        tag   : integer range 0 to 15;
    end record data_record;
    
    -- Access type (pointer)
    type int_ptr is access integer;
    
    -- File type
    type text_file is file of string;
    
    -- Alias
    alias slv is std_logic_vector;
    
    -- Function declarations
    function log2(n : positive) return natural;
    function max(a, b : integer) return integer;
    function "+"(a, b : byte) return byte;  -- Operator overloading
    
    -- Procedure declarations
    procedure reset_signals(signal clk : in std_logic; signal rst : out std_logic);
    
    -- Component declaration
    component counter
        generic (
            WIDTH : positive := 8
        );
        port (
            clk   : in  std_logic;
            rst   : in  std_logic;
            en    : in  std_logic;
            count : out std_logic_vector(WIDTH-1 downto 0)
        );
    end component counter;
    
    -- VHDL-2008: Package generics
    generic (
        type element_type;
        function compare(a, b : element_type) return boolean
    );
    
end package my_package;

-- =========================
-- Package Body
-- =========================
package body my_package is
    
    function log2(n : positive) return natural is
        variable result : natural := 0;
        variable value  : positive := n;
    begin
        while value > 1 loop
            value := value / 2;
            result := result + 1;
        end loop;
        return result;
    end function log2;
    
    function max(a, b : integer) return integer is
    begin
        if a > b then
            return a;
        else
            return b;
        end if;
    end function max;
    
    function "+"(a, b : byte) return byte is
    begin
        return std_logic_vector(unsigned(a) + unsigned(b));
    end function "+";
    
    procedure reset_signals(signal clk : in std_logic; signal rst : out std_logic) is
    begin
        rst <= '1';
        wait until rising_edge(clk);
        wait until rising_edge(clk);
        rst <= '0';
    end procedure reset_signals;
    
    -- Impure function (VHDL-2008)
    impure function get_random return real is
        variable seed1, seed2 : positive := 1;
        variable result : real;
    begin
        uniform(seed1, seed2, result);
        return result;
    end function get_random;
    
end package body my_package;

-- =========================
-- Entity Declaration
-- =========================
entity kitchen_sink is
    generic (
        DATA_WIDTH : positive := 8;
        DEPTH      : positive := 16;
        USE_RESET  : boolean := true;
        INIT_VALUE : std_logic_vector := x"00"
    );
    port (
        -- Clock and reset
        clk     : in  std_logic;
        rst_n   : in  std_logic;
        
        -- Input ports
        data_in  : in  std_logic_vector(DATA_WIDTH-1 downto 0);
        valid_in : in  std_logic;
        ready_in : out std_logic;
        
        -- Output ports
        data_out  : out std_logic_vector(DATA_WIDTH-1 downto 0);
        valid_out : out std_logic;
        ready_out : in  std_logic;
        
        -- Bidirectional port
        bidir : inout std_logic_vector(7 downto 0);
        
        -- Buffer port
        status : buffer std_logic_vector(3 downto 0)
    );
begin
    -- Entity statements (passive processes, assertions)
    assert DATA_WIDTH > 0 report "DATA_WIDTH must be positive" severity failure;
end entity kitchen_sink;

-- =========================
-- Architecture
-- =========================
architecture rtl of kitchen_sink is
    
    -- Signal declarations
    signal state, next_state : state_type;
    signal counter : unsigned(7 downto 0);
    signal data_reg : std_logic_vector(DATA_WIDTH-1 downto 0);
    signal fifo_mem : mem_type;
    signal wr_ptr, rd_ptr : integer range 0 to DEPTH-1;
    
    -- VHDL-2008: Signal with default value
    signal enable : std_logic := '0';
    
    -- Attribute declarations
    attribute mark_debug : string;
    attribute mark_debug of counter : signal is "true";
    
    attribute keep : boolean;
    attribute keep of data_reg : signal is true;
    
    -- Constants
    constant ZERO : std_logic_vector(DATA_WIDTH-1 downto 0) := (others => '0');
    constant ONES : std_logic_vector(DATA_WIDTH-1 downto 0) := (others => '1');
    
    -- Aliases
    alias upper_nibble : std_logic_vector(3 downto 0) is data_reg(7 downto 4);
    alias lower_nibble : std_logic_vector(3 downto 0) is data_reg(3 downto 0);
    
    -- Component instantiation (alternative declaration)
    component adder is
        port (
            a, b : in  std_logic_vector(7 downto 0);
            sum  : out std_logic_vector(8 downto 0)
        );
    end component;
    
    -- Function inside architecture
    function parity(vec : std_logic_vector) return std_logic is
        variable result : std_logic := '0';
    begin
        for i in vec'range loop
            result := result xor vec(i);
        end loop;
        return result;
    end function parity;
    
begin
    
    -- =========================
    -- Concurrent Statements
    -- =========================
    
    -- Simple signal assignment
    ready_in <= not fifo_full;
    
    -- Conditional signal assignment (VHDL-2008 style)
    data_out <= data_reg when valid_out = '1' else (others => 'Z');
    
    -- Selected signal assignment
    with state select
        status <= "0001" when IDLE,
                  "0010" when START,
                  "0100" when RUN,
                  "1000" when STOP,
                  "1111" when others;
    
    -- VHDL-2008: Conditional signal assignment with force
    data_out <= force data_reg when debug_mode = '1';
    data_out <= release when debug_mode = '0';
    
    -- Generate statements
    gen_pipeline : for i in 0 to 3 generate
        signal stage_data : std_logic_vector(7 downto 0);
    begin
        stage_reg : process(clk)
        begin
            if rising_edge(clk) then
                stage_data <= data_in;
            end if;
        end process;
    end generate gen_pipeline;
    
    -- Conditional generate (VHDL-2008)
    gen_reset : if USE_RESET generate
        rst_sync : process(clk)
        begin
            if rising_edge(clk) then
                rst_d1 <= rst_n;
                rst_d2 <= rst_d1;
            end if;
        end process;
    else generate
        rst_d2 <= '1';
    end generate gen_reset;
    
    -- Case generate (VHDL-2008)
    gen_width : case DATA_WIDTH generate
        when 8 =>
            byte_logic : process(clk) begin end process;
        when 16 =>
            word_logic : process(clk) begin end process;
        when others =>
            default_logic : process(clk) begin end process;
    end generate gen_width;
    
    -- Component instantiation (positional)
    adder_inst1 : adder port map (a, b, sum);
    
    -- Component instantiation (named)
    adder_inst2 : adder
        port map (
            a   => operand_a,
            b   => operand_b,
            sum => result
        );
    
    -- Direct entity instantiation (VHDL-93)
    counter_inst : entity work.counter(rtl)
        generic map (
            WIDTH => 8
        )
        port map (
            clk   => clk,
            rst   => rst,
            en    => enable,
            count => count_out
        );
    
    -- Block statement
    control_block : block
        signal local_sig : std_logic;
    begin
        local_sig <= clk and enable;
    end block control_block;
    
    -- Guarded block
    guarded_block : block (clk'event and clk = '1') is
    begin
        data_reg <= guarded data_in;
    end block guarded_block;
    
    -- =========================
    -- Process Statements
    -- =========================
    
    -- Combinational process (sensitivity list)
    comb_proc : process(state, data_in, valid_in)
    begin
        next_state <= state;
        case state is
            when IDLE =>
                if valid_in = '1' then
                    next_state <= START;
                end if;
            when START =>
                next_state <= RUN;
            when RUN =>
                if done = '1' then
                    next_state <= STOP;
                end if;
            when STOP =>
                next_state <= IDLE;
            when others =>
                next_state <= IDLE;
        end case;
    end process comb_proc;
    
    -- VHDL-2008: Process with "all" sensitivity
    comb_proc2 : process(all)
    begin
        result <= a + b;
    end process comb_proc2;
    
    -- Sequential process
    seq_proc : process(clk, rst_n)
    begin
        if rst_n = '0' then
            state <= IDLE;
            counter <= (others => '0');
        elsif rising_edge(clk) then
            state <= next_state;
            if enable = '1' then
                counter <= counter + 1;
            end if;
        end if;
    end process seq_proc;
    
    -- Process with variables
    var_proc : process(clk)
        variable temp : integer := 0;
        variable acc  : unsigned(15 downto 0);
    begin
        if rising_edge(clk) then
            temp := to_integer(unsigned(data_in));
            acc := acc + temp;
            result <= std_logic_vector(acc(7 downto 0));
        end if;
    end process var_proc;
    
    -- Process with wait statements
    wait_proc : process
    begin
        wait until clk = '1';
        wait for 10 ns;
        wait on data_in, valid_in;
        wait until rising_edge(clk) for 100 ns;
    end process wait_proc;
    
    -- Process with loop statements
    loop_proc : process(clk)
        variable i : integer;
    begin
        if rising_edge(clk) then
            -- For loop
            for i in 0 to 7 loop
                shift_reg(i+1) <= shift_reg(i);
            end loop;
            
            -- While loop
            i := 0;
            while i < 8 loop
                data(i) <= '0';
                i := i + 1;
            end loop;
            
            -- Infinite loop with exit
            loop
                count := count + 1;
                exit when count = 10;
            end loop;
            
            -- Loop with next
            for j in 0 to 15 loop
                next when skip(j) = '1';
                process_data(j);
            end loop;
        end if;
    end process loop_proc;
    
    -- =========================
    -- Expressions and Operators
    -- =========================
    
    expr_proc : process(all)
        variable a, b, c : integer;
        variable x, y, z : std_logic;
        variable vec : std_logic_vector(7 downto 0);
    begin
        -- Arithmetic operators
        a := b + c;
        a := b - c;
        a := b * c;
        a := b / c;
        a := b mod c;
        a := b rem c;
        a := abs(b);
        a := b ** 2;
        a := -b;
        
        -- Logical operators
        x := y and z;
        x := y or z;
        x := y xor z;
        x := y nand z;
        x := y nor z;
        x := y xnor z;
        x := not y;
        
        -- Relational operators
        x := '1' when a = b else '0';
        x := '1' when a /= b else '0';
        x := '1' when a < b else '0';
        x := '1' when a <= b else '0';
        x := '1' when a > b else '0';
        x := '1' when a >= b else '0';
        
        -- VHDL-2008: Matching relational operators
        x := '1' when vec ?= "1010----" else '0';
        x := '1' when vec ?/= "1010----" else '0';
        x := '1' when vec ?< "10000000" else '0';
        x := '1' when vec ?<= "10000000" else '0';
        x := '1' when vec ?> "10000000" else '0';
        x := '1' when vec ?>= "10000000" else '0';
        
        -- Shift operators
        vec := vec sll 2;
        vec := vec srl 2;
        vec := vec sla 1;
        vec := vec sra 1;
        vec := vec rol 3;
        vec := vec ror 3;
        
        -- Concatenation
        vec := "1010" & "0101";
        vec := x & "0000000";
        
        -- VHDL-2008: Condition operator
        x := ?? vec(0);
        
    end process expr_proc;
    
    -- =========================
    -- Aggregates
    -- =========================
    
    agg_proc : process(all)
        type point is record x, y : integer; end record;
        variable p : point;
        variable arr : int_array(0 to 3);
        variable vec : std_logic_vector(7 downto 0);
    begin
        -- Positional aggregate
        p := (10, 20);
        arr := (1, 2, 3, 4);
        
        -- Named aggregate
        p := (x => 10, y => 20);
        arr := (0 => 1, 1 => 2, 2 => 3, 3 => 4);
        
        -- Others aggregate
        vec := (others => '0');
        vec := (7 => '1', others => '0');
        arr := (0 | 2 => 1, others => 0);
        
        -- Range in aggregate
        vec := (7 downto 4 => '1', 3 downto 0 => '0');
        
    end process agg_proc;
    
    -- =========================
    -- Attributes
    -- =========================
    
    attr_proc : process(all)
        variable len : integer;
        variable hi, lo : integer;
    begin
        -- Array attributes
        len := data_in'length;
        hi := data_in'high;
        lo := data_in'low;
        len := data_in'left;
        len := data_in'right;
        
        -- Range attributes
        for i in data_in'range loop
        end loop;
        for i in data_in'reverse_range loop
        end loop;
        
        -- Signal attributes
        if clk'event and clk = '1' then
        end if;
        if data_in'stable(10 ns) then
        end if;
        if data_in'quiet(10 ns) then
        end if;
        delayed_data <= data_in'delayed(5 ns);
        
        -- Type attributes
        state <= state_type'val(0);
        idx := state_type'pos(IDLE);
        next_st := state_type'succ(state);
        prev_st := state_type'pred(state);
        first_st := state_type'leftof(state);
        last_st := state_type'rightof(state);
        
        -- VHDL-2008 attributes
        img := integer'image(42);
        val := integer'value("42");
        
    end process attr_proc;
    
    -- =========================
    -- Assertions and Reports
    -- =========================
    
    -- Concurrent assertion
    assert valid_in = '0' or ready_in = '1'
        report "Data lost: valid high but not ready"
        severity warning;
    
    -- PSL assertion (VHDL-2008)
    -- psl default clock is rising_edge(clk);
    -- psl assert always (req -> eventually! ack);
    
    -- Process with assertions
    assert_proc : process(clk)
    begin
        if rising_edge(clk) then
            assert counter < 256
                report "Counter overflow at time " & time'image(now)
                severity error;
            
            report "Simulation time: " & time'image(now);
        end if;
    end process assert_proc;
    
    -- =========================
    -- VHDL-2008 Features
    -- =========================
    
    -- External names (hierarchical references)
    ext_sig <= << signal .testbench.dut.internal_sig : std_logic >>;
    << signal .testbench.dut.internal_reg : std_logic_vector >> <= x"FF";
    
    -- VHDL-2008: Simplified sensitivity list
    simplified_proc : process(all)
    begin
        y <= a and b and c;
    end process;
    
    -- VHDL-2008: Enhanced bit string literals
    vec8 <= 8x"FF";
    vec8 <= 8ux"FF";
    vec8 <= 8sx"FF";
    vec12 <= 12x"ABC";
    
    -- VHDL-2008: Array slicing with 'subtype
    subvec <= vec(vec'subtype);
    
end architecture rtl;

-- =========================
-- Configuration
-- =========================
configuration kitchen_sink_cfg of kitchen_sink is
    for rtl
        for counter_inst : counter
            use entity work.counter(behavioral);
        end for;
        for all : adder
            use entity work.adder(rtl);
        end for;
        for gen_pipeline
            for stage_reg : process
            end for;
        end for;
    end for;
end configuration kitchen_sink_cfg;

-- =========================
-- Context (VHDL-2008)
-- =========================
context my_context is
    library ieee;
    use ieee.std_logic_1164.all;
    use ieee.numeric_std.all;
    library work;
    use work.my_package.all;
end context my_context;

-- =========================
-- Protected Types (for shared variables)
-- =========================
package protected_pkg is
    type shared_counter is protected
        procedure increment;
        procedure decrement;
        impure function get_value return integer;
    end protected shared_counter;
end package protected_pkg;

package body protected_pkg is
    type shared_counter is protected body
        variable count : integer := 0;
        
        procedure increment is
        begin
            count := count + 1;
        end procedure;
        
        procedure decrement is
        begin
            count := count - 1;
        end procedure;
        
        impure function get_value return integer is
        begin
            return count;
        end function;
    end protected body shared_counter;
end package body protected_pkg;

-- =========================
-- Testbench Example
-- =========================
entity kitchen_sink_tb is
end entity kitchen_sink_tb;

architecture sim of kitchen_sink_tb is
    signal clk : std_logic := '0';
    signal rst_n : std_logic := '0';
    signal data_in, data_out : std_logic_vector(7 downto 0);
    signal valid_in, valid_out : std_logic;
    signal ready_in, ready_out : std_logic;
    
    constant CLK_PERIOD : time := 10 ns;
    
    -- Shared variable (VHDL-2008 requires protected type)
    shared variable sv_counter : shared_counter;
    
    -- File I/O
    file log_file : text;
    
begin
    
    -- Clock generation
    clk <= not clk after CLK_PERIOD / 2;
    
    -- DUT instantiation
    dut : entity work.kitchen_sink(rtl)
        generic map (
            DATA_WIDTH => 8,
            DEPTH => 16
        )
        port map (
            clk => clk,
            rst_n => rst_n,
            data_in => data_in,
            valid_in => valid_in,
            ready_in => ready_in,
            data_out => data_out,
            valid_out => valid_out,
            ready_out => ready_out,
            bidir => open,
            status => open
        );
    
    -- Stimulus process
    stim_proc : process
        variable line_out : line;
        variable rand_val : real;
        variable seed1, seed2 : positive := 1;
    begin
        -- File operations
        file_open(log_file, "sim.log", write_mode);
        
        -- Initialize
        rst_n <= '0';
        data_in <= (others => '0');
        valid_in <= '0';
        ready_out <= '1';
        
        wait for CLK_PERIOD * 5;
        rst_n <= '1';
        
        -- Test loop
        for i in 0 to 255 loop
            wait until rising_edge(clk);
            data_in <= std_logic_vector(to_unsigned(i, 8));
            valid_in <= '1';
            
            wait until rising_edge(clk) and ready_in = '1';
            valid_in <= '0';
            
            -- Random delay
            uniform(seed1, seed2, rand_val);
            wait for integer(rand_val * 100.0) * 1 ns;
            
            -- Log to file
            write(line_out, string'("Sent data: "));
            write(line_out, i);
            writeline(log_file, line_out);
            
            -- Shared variable access
            sv_counter.increment;
        end loop;
        
        -- Report results
        report "Test complete. Transactions: " & integer'image(sv_counter.get_value);
        
        file_close(log_file);
        
        -- End simulation
        std.env.stop;
        wait;
    end process stim_proc;
    
    -- Monitor process
    monitor_proc : process(clk)
    begin
        if rising_edge(clk) then
            if valid_out = '1' and ready_out = '1' then
                report "Output: " & to_hstring(data_out);
            end if;
        end if;
    end process monitor_proc;
    
end architecture sim;
