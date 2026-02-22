import sys
import os
import subprocess
import tempfile

MUTATIONS = {
    'comb.and': ['comb.or', 'comb.xor'],
    'comb.or': ['comb.and', 'comb.xor'],
    'comb.xor': ['comb.and', 'comb.or'],
    'comb.add': ['comb.sub'],
    'comb.sub': ['comb.add'],
    'comb.icmp eq': ['comb.icmp ne'],
    'comb.icmp ne': ['comb.icmp eq'],
}

def get_btor2(mlir_path, circt_bin):
    cmd = rf"{circt_bin}/circt-opt {mlir_path} --lower-ltl-to-bmc -canonicalize --convert-hw-to-btor2 2>/dev/null | awk '/^module \{{/{{exit}} {{print}}'"
    res = subprocess.run(cmd, shell=True, capture_output=True, text=True)
    return res.stdout

def run_btormc(btor2_content):
    with tempfile.NamedTemporaryFile(mode='w', suffix='.btor2', delete=False) as f:
        f.write(btor2_content)
        tmp_name = f.name
        
    res = subprocess.run(["btormc", tmp_name], capture_output=True, text=True)
    os.remove(tmp_name)
    return res.stdout

def mutate(mlir_file, circt_bin):
    with open(mlir_file, 'r') as f:
        lines = f.readlines()
        
    print(f"Checking baseline for {mlir_file}...")
    base_btor2 = get_btor2(mlir_file, circt_bin)
    if not base_btor2.strip():
        print("ERROR: Failed to generate BTOR2 from MLIR.")
        return
        
    base_res = run_btormc(base_btor2)
    if "sat" in base_res and "unsat" not in base_res:
        print("ERROR: Baseline design fails its assertions. Fix the design first.")
        return
        
    print("Baseline passes. Starting mutation testing...")
    print()
    
    total = 0
    killed = 0
    survived = 0
    
    for i, line in enumerate(lines):
        # Prevent mutating constants, which might break the AST structurally
        if 'hw.constant' in line or 'hw.module' in line or 'verif.assert' in line:
            continue
            
        for orig, replacements in MUTATIONS.items():
            if orig in line:
                for rep in replacements:
                    mutated_line = line.replace(orig, rep)
                    mutated_lines = lines[:]
                    mutated_lines[i] = mutated_line
                    
                    with tempfile.NamedTemporaryFile(mode='w', suffix='.mlir', delete=False) as f:
                        f.writelines(mutated_lines)
                        mut_mlir = f.name
                        
                    mut_btor2 = get_btor2(mut_mlir, circt_bin)
                    if not mut_btor2.strip():
                        os.remove(mut_mlir)
                        continue
                        
                    mut_res = run_btormc(mut_btor2)
                    
                    total += 1
                    if "sat" in mut_res and "unsat" not in mut_res:
                        killed += 1
                        print(f"[\033[92mKILLED\033[0m] Line {i+1}: {orig} -> {rep}")
                    else:
                        survived += 1
                        print(f"[\033[91mSURVIVED\033[0m] Line {i+1}: {orig} -> {rep}")
                        print(f"  -> Mutated line: {mutated_line.strip()}")
                        
                    os.remove(mut_mlir)

    print()
    print(f"Mutation Score: {killed}/{total} ({(killed/total)*100 if total > 0 else 0:.1f}%)")
    if total == 0:
        print("No mutations could be applied.")
    elif survived > 0:
        print("Your verification suite is INCOMPLETE. Add more PSL assertions!")
    else:
        print("Perfect! Your assertions caught every single injected fault.")

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: python3 formal_mutation.py <file.mlir> <circt_bin_path>")
        sys.exit(1)
    mutate(sys.argv[1], sys.argv[2])