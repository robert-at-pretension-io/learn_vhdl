import sys, re, subprocess
mlir_file = sys.argv[1]
module = sys.argv[2]
bound = sys.argv[3]
circt_bmc = sys.argv[4]

with open(mlir_file) as f:
    content = f.read()

covers = list(re.finditer(r'(\s*)verif\.cover\s+(%[a-zA-Z0-9_]+)\s*:\s*i1', content))
if not covers:
    sys.exit(0)

print(f"  Found {len(covers)} cover properties to check.")

for i, match in enumerate(covers):
    indent = match.group(1)
    val = match.group(2)
    
    true_val = f"%true_cover_{i}"
    not_val = f"%not_cover_{i}"
    replacement = f"{indent}{true_val} = hw.constant -1 : i1\n{indent}{not_val} = comb.xor {val}, {true_val} : i1\n{indent}verif.assert {not_val} : i1"
    
    new_content = ""
    lines = content.split('\n')
    for line in lines:
        if 'verif.cover' in line and match.group(0).strip() in line:
            new_content += replacement + "\n"
        elif 'verif.assert' in line or 'verif.assume' in line or 'verif.cover' in line:
            pass # strip all other asserts, assumes, covers
        else:
            new_content += line + "\n"
            
    tmp_file = f"{mlir_file}.cover{i}.mlir"
    with open(tmp_file, "w") as f:
        f.write(new_content)
        
    res = subprocess.run([circt_bmc, tmp_file, f"--module={module}", "-b", bound, "--run", "--shared-libs=/usr/lib/x86_64-linux-gnu/libz3.so"], capture_output=True, text=True)
    if "Assertion can be violated!" in res.stdout:
        print(f"  Cover {i+1} REACHABLE! (Trace found)")
    else:
        print(f"  Cover {i+1} UNREACHABLE! (No trace found within bound {bound})")
