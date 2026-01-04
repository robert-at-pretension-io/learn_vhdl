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

-- CONVENTION: "ieee" and "work" are standard library names, but the library
-- clause itself just takes any identifier. "work" is a reserved implicit
-- library referring to the current working library.
-- LANGUAGE: library <identifier_list>; use <selected_name>;

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
    
    -- LANGUAGE: Bit string literals use base specifiers: x (hex), b (binary), o (octal)
    -- LANGUAGE: Physical literals have a unit (e.g., "10 ns") - space is optional
    -- CONVENTION: ALL_CAPS for constants is style, not required
    
    -- Type declarations
    type state_type is (IDLE, START, RUN, STOP, ERROR);
    -- LANGUAGE: Enumeration type - parenthesized list of identifiers
    -- CONVENTION: _type suffix is style only
    -- NOTE: ERROR is a valid user identifier, not a reserved word
    
    type int_array is array (natural range <>) of integer;
    -- LANGUAGE: Unconstrained array type uses "range <>" (box)
    
    type matrix is array (0 to 3, 0 to 3) of std_logic;
    -- LANGUAGE: Multi-dimensional array with multiple index ranges
    
    type mem_type is array (0 to 255) of std_logic_vector(7 downto 0);
    -- LANGUAGE: Array of arrays (not same as 2D array semantically)
    
    -- Subtypes
    subtype byte is std_logic_vector(7 downto 0);
    subtype nibble is std_logic_vector(3 downto 0);
    subtype small_int is integer range 0 to 100;
    -- LANGUAGE: subtype constrains an existing type
    -- CONVENTION: Names like "byte" are user choice, not predefined
    
    -- Record type
    type data_record is record
        valid : std_logic;
        data  : std_logic_vector(31 downto 0);
        tag   : integer range 0 to 15;
    end record data_record;
    -- LANGUAGE: Record fields are element declarations
    -- LANGUAGE: Trailing name after "end record" is optional
    
    -- Access type (pointer)
    type int_ptr is access integer;
    -- LANGUAGE: Access types are VHDL's pointers, rarely synthesizable
    
    -- File type
    type text_file is file of string;
    -- LANGUAGE: File types for file I/O
    
    -- Alias
    alias slv is std_logic_vector;
    -- LANGUAGE: Alias can rename types, objects, or subprograms
    -- NOTE: This aliases a type name, not an object
    
    -- Function declarations
    function log2(n : positive) return natural;
    function max(a, b : integer) return integer;
    -- LANGUAGE: "a, b : integer" declares both parameters with same type
    
    function "+"(a, b : byte) return byte;  -- Operator overloading
    -- LANGUAGE: Operators are overloadable via string literal names
    -- LANGUAGE: Valid operator names: "and" "or" "nand" "nor" "xor" "xnor"
    --           "=" "/=" "<" "<=" ">" ">=" "sll" "srl" "sla" "sra" "rol" "ror"
    --           "+" "-" "&" "*" "/" "mod" "rem" "**" "abs" "not"
    --           VHDL-2008 adds: "?=" "?/=" "?<" "?<=" "?>" "?>=" "??"
    
    -- Procedure declarations
    procedure reset_signals(signal clk : in std_logic; signal rst : out std_logic);
    -- LANGUAGE: "signal" keyword before parameter name indicates signal class
    -- LANGUAGE: Other classes: "variable", "constant", "file"
    -- LANGUAGE: Default class depends on mode: in=constant, out/inout=variable
    
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
    -- LANGUAGE: Component name after "end component" is optional
    -- CONVENTION: Matching component name to entity name is convention
    
    -- VHDL-2008: Package generics
    generic (
        type element_type;
        function compare(a, b : element_type) return boolean
    );
    -- LANGUAGE: VHDL-2008 generic types and generic subprograms
    -- LANGUAGE: "type element_type;" declares a generic type parameter
    
end package my_package;
-- LANGUAGE: "my_package" after "end package" is optional

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
    -- LANGUAGE: "function" and "log2" after "end" are both optional
    -- LANGUAGE: Can be just "end;" or "end function;" or "end log2;"
    
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
        -- CONVENTION: unsigned() is from ieee.numeric_std, not built-in
    end function "+";
    
    procedure reset_signals(signal clk : in std_logic; signal rst : out std_logic) is
    begin
        rst <= '1';
        wait until rising_edge(clk);
        -- CONVENTION: rising_edge() is from ieee.std_logic_1164, not built-in
        -- LANGUAGE: Equivalent to: clk'event and clk = '1'
        wait until rising_edge(clk);
        rst <= '0';
    end procedure reset_signals;
    
    -- Impure function (VHDL-2008)
    impure function get_random return real is
        variable seed1, seed2 : positive := 1;
        variable result : real;
    begin
        uniform(seed1, seed2, result);
        -- CONVENTION: uniform() is from ieee.math_real
        return result;
    end function get_random;
    -- LANGUAGE: "impure" keyword indicates function may have side effects
    -- LANGUAGE: Pure functions (default) cannot access signals or files
    
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
        -- LANGUAGE: Unconstrained generic type gets size from actual
    );
    port (
        -- Clock and reset
        clk     : in  std_logic;
        rst_n   : in  std_logic;
        -- CONVENTION: _n suffix for active-low is style only
        -- CONVENTION: "clk" and "rst" names are convention
        
        -- Input ports
        data_in  : in  std_logic_vector(DATA_WIDTH-1 downto 0);
        valid_in : in  std_logic;
        ready_in : out std_logic;
        -- CONVENTION: _in/_out suffixes are style only
        
        -- Output ports
        data_out  : out std_logic_vector(DATA_WIDTH-1 downto 0);
        valid_out : out std_logic;
        ready_out : in  std_logic;
        
        -- Bidirectional port
        bidir : inout std_logic_vector(7 downto 0);
        -- LANGUAGE: "inout" is the mode for bidirectional ports
        
        -- Buffer port
        status : buffer std_logic_vector(3 downto 0)
        -- LANGUAGE: "buffer" mode: output that can be read internally
        -- LANGUAGE: Modes are: in, out, inout, buffer, linkage
    );
begin
    -- Entity statements (passive processes, assertions)
    assert DATA_WIDTH > 0 report "DATA_WIDTH must be positive" severity failure;
    -- LANGUAGE: Entity statement part is optional, only passive statements allowed
    -- LANGUAGE: Severity levels: note, warning, error, failure
end entity kitchen_sink;

-- =========================
-- Architecture
-- =========================
architecture rtl of kitchen_sink is
    -- CONVENTION: "rtl" is a common architecture name, not required
    -- CONVENTION: Other common names: "behavioral", "structural", "sim"
    
    -- Signal declarations
    signal state, next_state : state_type;
    signal counter : unsigned(7 downto 0);
    -- NOTE: "counter" shadows the component name - legal but confusing
    signal data_reg : std_logic_vector(DATA_WIDTH-1 downto 0);
    signal fifo_mem : mem_type;
    signal wr_ptr, rd_ptr : integer range 0 to DEPTH-1;
    
    -- VHDL-2008: Signal with default value
    signal enable : std_logic := '0';
    -- LANGUAGE: Default values on signals are VHDL-2008
    -- LANGUAGE: Pre-2008, only constants and variables could have defaults
    
    -- Attribute declarations
    attribute mark_debug : string;
    attribute mark_debug of counter : signal is "true";
    -- LANGUAGE: User-defined attributes - declaration then specification
    -- CONVENTION: "mark_debug" is Xilinx-specific, not standard VHDL
    
    attribute keep : boolean;
    attribute keep of data_reg : signal is true;
    -- CONVENTION: "keep" is vendor-specific synthesis attribute
    
    -- Constants
    constant ZERO : std_logic_vector(DATA_WIDTH-1 downto 0) := (others => '0');
    constant ONES : std_logic_vector(DATA_WIDTH-1 downto 0) := (others => '1');
    -- LANGUAGE: (others => '0') is an aggregate with "others" choice
    
    -- Aliases
    alias upper_nibble : std_logic_vector(3 downto 0) is data_reg(7 downto 4);
    alias lower_nibble : std_logic_vector(3 downto 0) is data_reg(3 downto 0);
    -- LANGUAGE: Alias creates alternative name for slice of object
    
    -- Component instantiation (alternative declaration)
    component adder is
        port (
            a, b : in  std_logic_vector(7 downto 0);
            sum  : out std_logic_vector(8 downto 0)
        );
    end component;
    -- LANGUAGE: "is" after component name is optional (VHDL-93+)
    -- LANGUAGE: Component name after "end component" is optional
    
    -- Function inside architecture
    function parity(vec : std_logic_vector) return std_logic is
        variable result : std_logic := '0';
    begin
        for i in vec'range loop
            result := result xor vec(i);
        end loop;
        return result;
    end function parity;
    -- LANGUAGE: Functions can be declared in architecture declarative region
    
begin
    
    -- =========================
    -- Concurrent Statements
    -- =========================
    
    -- Simple signal assignment
    ready_in <= not fifo_full;
    -- NOTE: fifo_full is not declared - this would be an error
    -- LANGUAGE: Concurrent signal assignment (outside process)
    
    -- Conditional signal assignment (VHDL-2008 style)
    data_out <= data_reg when valid_out = '1' else (others => 'Z');
    -- LANGUAGE: Concurrent conditional signal assignment
    -- LANGUAGE: 'Z' is high-impedance (std_logic value)
    
    -- Selected signal assignment
    with state select
        status <= "0001" when IDLE,
                  "0010" when START,
                  "0100" when RUN,
                  "1000" when STOP,
                  "1111" when others;
    -- LANGUAGE: Concurrent selected signal assignment
    -- LANGUAGE: "when others" is required if not all choices covered
    
    -- VHDL-2008: Conditional signal assignment with force
    data_out <= force data_reg when debug_mode = '1';
    data_out <= release when debug_mode = '0';
    -- LANGUAGE: VHDL-2008 force/release for simulation override
    -- NOTE: debug_mode is not declared
    
    -- Generate statements
    gen_pipeline : for i in 0 to 3 generate
        signal stage_data : std_logic_vector(7 downto 0);
        -- LANGUAGE: VHDL-2008 allows declarations in generate
        -- LANGUAGE: Pre-2008 required a nested block for local declarations
    begin
        stage_reg : process(clk)
        begin
            if rising_edge(clk) then
                stage_data <= data_in;
            end if;
        end process;
    end generate gen_pipeline;
    -- LANGUAGE: Generate label before "for" is required
    -- LANGUAGE: Label after "end generate" is optional
    
    -- Conditional generate (VHDL-2008)
    gen_reset : if USE_RESET generate
        rst_sync : process(clk)
        begin
            if rising_edge(clk) then
                rst_d1 <= rst_n;
                rst_d2 <= rst_d1;
                -- NOTE: rst_d1, rst_d2 not declared
            end if;
        end process;
    else generate
        rst_d2 <= '1';
    end generate gen_reset;
    -- LANGUAGE: VHDL-2008 adds "else generate" and "elsif generate"
    
    -- Case generate (VHDL-2008)
    gen_width : case DATA_WIDTH generate
        when 8 =>
            byte_logic : process(clk) begin end process;
        when 16 =>
            word_logic : process(clk) begin end process;
        when others =>
            default_logic : process(clk) begin end process;
    end generate gen_width;
    -- LANGUAGE: VHDL-2008 case generate statement
    
    -- Component instantiation (positional)
    adder_inst1 : adder port map (a, b, sum);
    -- LANGUAGE: Positional association in port map
    -- NOTE: a, b, sum not declared as signals
    
    -- Component instantiation (named)
    adder_inst2 : adder
        port map (
            a   => operand_a,
            b   => operand_b,
            sum => result
        );
    -- LANGUAGE: Named association uses "=>"
    -- NOTE: operand_a, operand_b, result not declared
    
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
    -- LANGUAGE: Direct instantiation bypasses component declaration
    -- LANGUAGE: Architecture name in parentheses is optional
    -- NOTE: rst, count_out not declared
    
    -- Block statement
    control_block : block
        signal local_sig : std_logic;
    begin
        local_sig <= clk and enable;
    end block control_block;
    -- LANGUAGE: Block creates local declarative region
    -- LANGUAGE: Block label is required, trailing label optional
    
    -- Guarded block
    guarded_block : block (clk'event and clk = '1') is
        -- LANGUAGE: Guard expression in parentheses
    begin
        data_reg <= guarded data_in;
        -- LANGUAGE: "guarded" keyword makes assignment conditional on guard
    end block guarded_block;
    -- LANGUAGE: "is" after guard expression is optional
    
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
                    -- NOTE: done not declared
                    next_state <= STOP;
                end if;
            when STOP =>
                next_state <= IDLE;
            when others =>
                next_state <= IDLE;
        end case;
    end process comb_proc;
    -- LANGUAGE: Process label is optional
    -- LANGUAGE: Trailing label after "end process" is optional
    -- CONVENTION: Naming processes is style, not required
    
    -- VHDL-2008: Process with "all" sensitivity
    comb_proc2 : process(all)
    begin
        result <= a + b;
        -- NOTE: result, a, b not declared
    end process comb_proc2;
    -- LANGUAGE: VHDL-2008 "all" means sensitive to all signals read
    
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
    -- CONVENTION: Async reset pattern (rst in sensitivity list) is style
    -- LANGUAGE: elsif is a keyword (not "else if")
    
    -- Process with variables
    var_proc : process(clk)
        variable temp : integer := 0;
        variable acc  : unsigned(15 downto 0);
    begin
        if rising_edge(clk) then
            temp := to_integer(unsigned(data_in));
            -- CONVENTION: to_integer() is from ieee.numeric_std
            acc := acc + temp;
            result <= std_logic_vector(acc(7 downto 0));
        end if;
    end process var_proc;
    -- LANGUAGE: Variables use := for assignment, signals use <=
    
    -- Process with wait statements
    wait_proc : process
    begin
        wait until clk = '1';
        wait for 10 ns;
        wait on data_in, valid_in;
        wait until rising_edge(clk) for 100 ns;
        -- LANGUAGE: wait until <condition> for <timeout>
    end process wait_proc;
    -- LANGUAGE: Process without sensitivity list must have wait statements
    -- LANGUAGE: Process with sensitivity list cannot have wait statements
    
    -- Process with loop statements
    loop_proc : process(clk)
        variable i : integer;
    begin
        if rising_edge(clk) then
            -- For loop
            for i in 0 to 7 loop
                shift_reg(i+1) <= shift_reg(i);
                -- NOTE: shift_reg not declared
            end loop;
            -- LANGUAGE: Loop variable "i" is implicitly declared, shadows outer i
            
            -- While loop
            i := 0;
            while i < 8 loop
                data(i) <= '0';
                -- NOTE: data not declared
                i := i + 1;
            end loop;
            
            -- Infinite loop with exit
            loop
                count := count + 1;
                -- NOTE: count not declared as variable
                exit when count = 10;
            end loop;
            -- LANGUAGE: Bare "loop" is infinite loop
            
            -- Loop with next
            for j in 0 to 15 loop
                next when skip(j) = '1';
                -- NOTE: skip not declared
                process_data(j);
                -- NOTE: process_data not declared
            end loop;
            -- LANGUAGE: "next" skips to next iteration
            -- LANGUAGE: "next when <condition>" is conditional skip
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
        a := b mod c;  -- LANGUAGE: Modulus (sign of divisor)
        a := b rem c;  -- LANGUAGE: Remainder (sign of dividend)
        a := abs(b);   -- LANGUAGE: Absolute value (unary)
        a := b ** 2;   -- LANGUAGE: Exponentiation
        a := -b;       -- LANGUAGE: Negation (unary)
        
        -- Logical operators
        x := y and z;
        x := y or z;
        x := y xor z;
        x := y nand z;
        x := y nor z;
        x := y xnor z;  -- LANGUAGE: xnor added in VHDL-93
        x := not y;     -- LANGUAGE: Unary not
        
        -- Relational operators
        x := '1' when a = b else '0';
        x := '1' when a /= b else '0';  -- LANGUAGE: /= is not-equal
        x := '1' when a < b else '0';
        x := '1' when a <= b else '0';
        x := '1' when a > b else '0';
        x := '1' when a >= b else '0';
        
        -- VHDL-2008: Matching relational operators
        x := '1' when vec ?= "1010----" else '0';   -- LANGUAGE: Matching equal
        x := '1' when vec ?/= "1010----" else '0';  -- LANGUAGE: Matching not-equal
        x := '1' when vec ?< "10000000" else '0';   -- LANGUAGE: Matching less
        x := '1' when vec ?<= "10000000" else '0';  -- LANGUAGE: Matching less-equal
        x := '1' when vec ?> "10000000" else '0';   -- LANGUAGE: Matching greater
        x := '1' when vec ?>= "10000000" else '0';  -- LANGUAGE: Matching greater-equal
        -- LANGUAGE: Matching operators treat '-' as don't-care
        
        -- Shift operators
        vec := vec sll 2;  -- LANGUAGE: Shift left logical
        vec := vec srl 2;  -- LANGUAGE: Shift right logical
        vec := vec sla 1;  -- LANGUAGE: Shift left arithmetic
        vec := vec sra 1;  -- LANGUAGE: Shift right arithmetic
        vec := vec rol 3;  -- LANGUAGE: Rotate left
        vec := vec ror 3;  -- LANGUAGE: Rotate right
        
        -- Concatenation
        vec := "1010" & "0101";
        vec := x & "0000000";
        -- LANGUAGE: & is concatenation operator
        
        -- VHDL-2008: Condition operator
        x := ?? vec(0);
        -- LANGUAGE: ?? converts std_ulogic to boolean ('1'/'H' = true)
        
    end process expr_proc;
    
    -- =========================
    -- Aggregates
    -- =========================
    
    agg_proc : process(all)
        type point is record x, y : integer; end record;
        -- LANGUAGE: Type can be declared inside process
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
        -- LANGUAGE: | separates multiple choices
        
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
        len := data_in'length;   -- LANGUAGE: Number of elements
        hi := data_in'high;      -- LANGUAGE: Upper bound
        lo := data_in'low;       -- LANGUAGE: Lower bound
        len := data_in'left;     -- LANGUAGE: Leftmost index
        len := data_in'right;    -- LANGUAGE: Rightmost index
        
        -- Range attributes
        for i in data_in'range loop
        end loop;
        for i in data_in'reverse_range loop
        end loop;
        -- LANGUAGE: 'range and 'reverse_range return index range
        
        -- Signal attributes
        if clk'event and clk = '1' then
        -- LANGUAGE: 'event is true if signal changed this delta cycle
        end if;
        if data_in'stable(10 ns) then
        -- LANGUAGE: 'stable(T) is true if no event in last T time
        end if;
        if data_in'quiet(10 ns) then
        -- LANGUAGE: 'quiet(T) is true if no transaction in last T time
        end if;
        delayed_data <= data_in'delayed(5 ns);
        -- LANGUAGE: 'delayed(T) returns signal delayed by T
        -- NOTE: delayed_data not declared
        
        -- Type attributes
        state <= state_type'val(0);
        -- LANGUAGE: 'val(N) returns Nth value of enumeration
        idx := state_type'pos(IDLE);
        -- LANGUAGE: 'pos(V) returns position of value V
        -- NOTE: idx not declared
        next_st := state_type'succ(state);
        -- LANGUAGE: 'succ returns successor value
        -- NOTE: next_st not declared
        prev_st := state_type'pred(state);
        -- LANGUAGE: 'pred returns predecessor value
        -- NOTE: prev_st not declared
        first_st := state_type'leftof(state);
        -- LANGUAGE: 'leftof returns value to the left
        -- NOTE: first_st not declared
        last_st := state_type'rightof(state);
        -- LANGUAGE: 'rightof returns value to the right
        -- NOTE: last_st not declared
        
        -- VHDL-2008 attributes
        img := integer'image(42);
        -- LANGUAGE: 'image returns string representation
        -- NOTE: img not declared
        val := integer'value("42");
        -- LANGUAGE: 'value parses string to value
        -- NOTE: val not declared (also shadows outer val)
        
    end process attr_proc;
    
    -- =========================
    -- Assertions and Reports
    -- =========================
    
    -- Concurrent assertion
    assert valid_in = '0' or ready_in = '1'
        report "Data lost: valid high but not ready"
        severity warning;
    -- LANGUAGE: Concurrent assertion (outside process)
    -- LANGUAGE: "report" and "severity" clauses are optional
    -- LANGUAGE: Default severity is "error"
    
    -- PSL assertion (VHDL-2008)
    -- psl default clock is rising_edge(clk);
    -- psl assert always (req -> eventually! ack);
    -- LANGUAGE: PSL is embedded in comments with "psl" prefix
    -- LANGUAGE: PSL is optional part of VHDL-2008 standard
    
    -- Process with assertions
    assert_proc : process(clk)
    begin
        if rising_edge(clk) then
            assert counter < 256
                report "Counter overflow at time " & time'image(now)
                severity error;
            -- LANGUAGE: Sequential assertion (inside process)
            -- LANGUAGE: "now" is predefined function returning current sim time
            
            report "Simulation time: " & time'image(now);
            -- LANGUAGE: Report statement without assertion
        end if;
    end process assert_proc;
    
    -- =========================
    -- VHDL-2008 Features
    -- =========================
    
    -- External names (hierarchical references)
    ext_sig <= << signal .testbench.dut.internal_sig : std_logic >>;
    -- LANGUAGE: VHDL-2008 external name (hierarchical path)
    -- LANGUAGE: << >> delimiters, path starts with . for absolute
    -- NOTE: ext_sig not declared
    << signal .testbench.dut.internal_reg : std_logic_vector >> <= x"FF";
    -- LANGUAGE: External names can be targets of assignments
    
    -- VHDL-2008: Simplified sensitivity list
    simplified_proc : process(all)
    begin
        y <= a and b and c;
        -- NOTE: y, a, b, c not declared
    end process;
    -- LANGUAGE: Trailing label after "end process" is optional
    
    -- VHDL-2008: Enhanced bit string literals
    vec8 <= 8x"FF";
    -- LANGUAGE: Length-specified bit string: <length><base>"<value>"
    -- NOTE: vec8 not declared
    vec8 <= 8ux"FF";
    -- LANGUAGE: 'u' prefix = unsigned (zero-extend)
    vec8 <= 8sx"FF";
    -- LANGUAGE: 's' prefix = signed (sign-extend)
    vec12 <= 12x"ABC";
    -- NOTE: vec12 not declared
    
    -- VHDL-2008: Array slicing with 'subtype
    subvec <= vec(vec'subtype);
    -- LANGUAGE: 'subtype returns the subtype of an object
    -- NOTE: subvec, vec not declared in this scope
    
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
        -- LANGUAGE: "all" matches all instances of component
        for gen_pipeline
            for stage_reg : process
            end for;
        end for;
    end for;
end configuration kitchen_sink_cfg;
-- LANGUAGE: Configuration binds components to entities
-- LANGUAGE: "kitchen_sink_cfg" after "end configuration" is optional

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
-- LANGUAGE: VHDL-2008 context declaration groups library/use clauses
-- LANGUAGE: Used via: context work.my_context;

-- =========================
-- Protected Types (for shared variables)
-- =========================
package protected_pkg is
    type shared_counter is protected
        procedure increment;
        procedure decrement;
        impure function get_value return integer;
    end protected shared_counter;
    -- LANGUAGE: Protected type declaration (interface)
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
    -- LANGUAGE: Protected type body (implementation)
end package body protected_pkg;
-- LANGUAGE: Protected types provide mutual exclusion for shared variables

-- =========================
-- Testbench Example
-- =========================
entity kitchen_sink_tb is
end entity kitchen_sink_tb;
-- LANGUAGE: Empty entity (no ports/generics) is valid
-- CONVENTION: _tb suffix for testbench is style only

architecture sim of kitchen_sink_tb is
    signal clk : std_logic := '0';
    signal rst_n : std_logic := '0';
    signal data_in, data_out : std_logic_vector(7 downto 0);
    signal valid_in, valid_out : std_logic;
    signal ready_in, ready_out : std_logic;
    
    constant CLK_PERIOD : time := 10 ns;
    
    -- Shared variable (VHDL-2008 requires protected type)
    shared variable sv_counter : shared_counter;
    -- LANGUAGE: "shared" keyword for variables accessible by multiple processes
    -- LANGUAGE: VHDL-2008 requires shared variables to be protected type
    
    -- File I/O
    file log_file : text;
    -- LANGUAGE: File declaration (text is from std.textio)
    -- CONVENTION: "text" type is from std.textio package
    
begin
    
    -- Clock generation
    clk <= not clk after CLK_PERIOD / 2;
    -- LANGUAGE: Concurrent signal assignment with "after" delay
    -- CONVENTION: This clock generation pattern is common but not required
    
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
    -- LANGUAGE: "open" leaves port unconnected
    -- CONVENTION: "dut" label is common convention for device under test
    
    -- Stimulus process
    stim_proc : process
        variable line_out : line;
        -- CONVENTION: "line" type is from std.textio
        variable rand_val : real;
        variable seed1, seed2 : positive := 1;
    begin
        -- File operations
        file_open(log_file, "sim.log", write_mode);
        -- LANGUAGE: file_open procedure opens file
        -- LANGUAGE: Modes: read_mode, write_mode, append_mode
        
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
            -- CONVENTION: to_unsigned() is from ieee.numeric_std
            valid_in <= '1';
            
            wait until rising_edge(clk) and ready_in = '1';
            valid_in <= '0';
            
            -- Random delay
            uniform(seed1, seed2, rand_val);
            -- CONVENTION: uniform() is from ieee.math_real
            wait for integer(rand_val * 100.0) * 1 ns;
            
            -- Log to file
            write(line_out, string'("Sent data: "));
            -- LANGUAGE: string'(...) is a qualified expression
            -- CONVENTION: write() is from std.textio
            write(line_out, i);
            writeline(log_file, line_out);
            -- CONVENTION: writeline() is from std.textio
            
            -- Shared variable access
            sv_counter.increment;
            -- LANGUAGE: Protected type method call
        end loop;
        
        -- Report results
        report "Test complete. Transactions: " & integer'image(sv_counter.get_value);
        
        file_close(log_file);
        -- LANGUAGE: file_close procedure closes file
        
        -- End simulation
        std.env.stop;
        -- LANGUAGE: VHDL-2008 std.env package provides stop/finish
        -- LANGUAGE: stop halts simulation, finish ends with status
        wait;
        -- LANGUAGE: Bare "wait" suspends process forever
    end process stim_proc;
    
    -- Monitor process
    monitor_proc : process(clk)
    begin
        if rising_edge(clk) then
            if valid_out = '1' and ready_out = '1' then
                report "Output: " & to_hstring(data_out);
                -- CONVENTION: to_hstring() is from ieee.std_logic_1164 (VHDL-2008)
                -- LANGUAGE: Pre-2008 would need custom function or textio
            end if;
        end if;
    end process monitor_proc;
    
end architecture sim;
