use std::error::Error;
use std::fs::File;
use std::io::{self, Read};

use vhdl_compiler::policy::engine;
use vhdl_compiler::policy::input::Input;

fn main() -> Result<(), Box<dyn Error>> {
    let args: Vec<String> = std::env::args().collect();
    let input = if args.len() > 1 {
        read_input_file(&args[1])?
    } else {
        read_input_stdin()?
    };

    let result = engine::evaluate(&input);
    serde_json::to_writer_pretty(std::io::stdout(), &result)?;
    Ok(())
}

fn read_input_file(path: &str) -> Result<Input, Box<dyn Error>> {
    let file = File::open(path)?;
    let input: Input = serde_json::from_reader(file)?;
    Ok(input)
}

fn read_input_stdin() -> Result<Input, Box<dyn Error>> {
    let mut buf = String::new();
    io::stdin().read_to_string(&mut buf)?;
    let input: Input = serde_json::from_str(&buf)?;
    Ok(input)
}
