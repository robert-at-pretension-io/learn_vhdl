const Parser = require('tree-sitter');
const MicroVHDL = require('./tree-sitter-micro-vhdl');

const parser = new Parser();
parser.setLanguage(MicroVHDL);

const sourceCode = `
entity Arbiter is
  port (req0, req1 : in std_logic; grant0, grant1 : out std_logic; clk : in std_logic);
  contract
    require: not (req0 = '1' and req1 = '1');
    ensure:  not (grant0 = '1' and grant1 = '1');
end entity;
`;

const tree = parser.parse(sourceCode);
console.log(tree.rootNode.toString());
