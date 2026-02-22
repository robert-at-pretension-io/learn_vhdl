import sys

def modify_aig(in_path, out_path):
    with open(in_path, 'rb') as f:
        data = f.read()
    header_end = data.find(b'\n')
    header = data[:header_end].decode('ascii')
    parts = header.split()
    M, I, L, O, A = parts[1:6]
    # new header: aig M I L 0 A 0 0 O 0
    new_header = f"aig {M} {I} {L} 0 {A} 0 0 {O} 0".encode('ascii')
    rest = data[header_end+1:]
    modified_rest = rest.replace(b'\no0', b'\nj0').replace(b'\no1', b'\nj1')
    with open(out_path, 'wb') as f:
        f.write(new_header + b'\n' + modified_rest)

modify_aig('test.aig', 'test_justice.aig')
