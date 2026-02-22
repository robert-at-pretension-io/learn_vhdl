import sys
import re
from datetime import datetime

def parse_btor_trace(trace_text):
    lines = trace_text.strip().splitlines()
    if not lines or lines[0] != 'sat':
        return None
        
    cycles = {} # cycle_num -> {'states': {}, 'inputs': {}}
    variables = {} # name -> {'width': w, 'symbol': s}
    
    current_cycle = -1
    section = None # 'state' or 'input'
    
    symbol_counter = 33 # start from '!'
    
    for line in lines[1:]:
        line = line.strip()
        if not line or line == '.':
            continue
            
        if line.startswith('b'):
            continue # bad property index
            
        if line.startswith('#'):
            current_cycle = int(line[1:])
            section = 'state'
            if current_cycle not in cycles:
                cycles[current_cycle] = {'states': {}, 'inputs': {}}
            continue
            
        if line.startswith('@'):
            current_cycle = int(line[1:])
            section = 'input'
            if current_cycle not in cycles:
                cycles[current_cycle] = {'states': {}, 'inputs': {}}
            continue
            
        # Parse value lines: <id> <value> <name>#<cycle> or <name>@<cycle>
        parts = line.split(maxsplit=2)
        if len(parts) == 3:
            val_id = parts[0]
            val_bin = parts[1]
            name_cyc = parts[2]
            
            # extract name and verify cycle
            if section == 'state':
                name, cyc = name_cyc.split('#')
            else:
                name, cyc = name_cyc.split('@')
                
            if int(cyc) != current_cycle:
                continue
                
            if name not in variables:
                variables[name] = {
                    'width': len(val_bin),
                    'symbol': chr(symbol_counter)
                }
                symbol_counter += 1
                
            if section == 'state':
                cycles[current_cycle]['states'][name] = val_bin
            else:
                cycles[current_cycle]['inputs'][name] = val_bin
                
    return variables, cycles
    
def write_vcd(variables, cycles, out_file):
    date_str = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    out_file.write(f"$date\n    {date_str}\n$end\n")
    out_file.write("$version\n    Micro-VHDL BTOR2VCD\n$end\n")
    out_file.write("$timescale\n    1ns\n$end\n")
    out_file.write("$scope module trace $end\n")
    
    # Add a clock signal
    clk_symbol = chr(len(variables) + 33)
    out_file.write(f"$var wire 1 {clk_symbol} clk $end\n")
    
    for name, info in variables.items():
        out_file.write(f"$var wire {info['width']} {info['symbol']} {name} $end\n")
        
    out_file.write("$upscope $end\n")
    out_file.write("$enddefinitions $end\n")
    
    max_cycle = max(cycles.keys()) if cycles else -1
    
    # Dump values per cycle
    # VCD convention:
    # 0 = rising edge, 5 = falling edge, 10 = next rising edge
    
    prev_vals = {}
    
    for c in range(max_cycle + 1):
        if c not in cycles:
            continue
            
        time_ns = c * 10
        
        # Rising edge (clock goes 1, inputs update, states represent current register value)
        out_file.write(f"#{time_ns}\n")
        out_file.write(f"1{clk_symbol}\n")
        
        # Dump inputs that changed
        for name, val in cycles[c]['inputs'].items():
            sym = variables[name]['symbol']
            if prev_vals.get(name) != val:
                if len(val) == 1:
                    out_file.write(f"{val}{sym}\n")
                else:
                    out_file.write(f"b{val} {sym}\n")
                prev_vals[name] = val
                
        # Dump states that changed
        for name, val in cycles[c]['states'].items():
            sym = variables[name]['symbol']
            if prev_vals.get(name) != val:
                if len(val) == 1:
                    out_file.write(f"{val}{sym}\n")
                else:
                    out_file.write(f"b{val} {sym}\n")
                prev_vals[name] = val
                
        # Falling edge (clock goes 0)
        time_falling = time_ns + 5
        out_file.write(f"#{time_falling}\n")
        out_file.write(f"0{clk_symbol}\n")
        
if __name__ == '__main__':
    if len(sys.argv) != 3:
        print("Usage: python3 btor2vcd.py <btor_trace.txt> <output.vcd>")
        sys.exit(1)
        
    trace_file = sys.argv[1]
    vcd_file = sys.argv[2]
    
    with open(trace_file, 'r') as f:
        trace_text = f.read()
        
    res = parse_btor_trace(trace_text)
    if not res:
        print(f"No SAT trace found in {trace_file}")
        sys.exit(1)
        
    variables, cycles = res
    with open(vcd_file, 'w') as f:
        write_vcd(variables, cycles, f)
        
    print(f"VCD trace written to {vcd_file}")
