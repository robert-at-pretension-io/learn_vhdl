use std::env;
use std::fs;

const MAX_ERRORS: usize = 10;

fn main() {
    let args: Vec<String> = env::args().collect();
    
    let filename = args.get(1).map(|s| s.as_str()).unwrap_or("test.vhdl");
    
    let source_code = match fs::read_to_string(filename) {
        Ok(content) => content,
        Err(e) => {
            eprintln!("Error reading '{}': {}", filename, e);
            std::process::exit(1);
        }
    };

    let mut parser = tree_sitter::Parser::new();
    parser
        .set_language(&tree_sitter_vhdl::language())
        .expect("Error loading VHDL grammar");

    let tree = parser.parse(&source_code, None).expect("Failed to parse");
    let root = tree.root_node();

    // Walk and report any errors (up to MAX_ERRORS)
    let mut error_count: usize = 0;
    let mut cursor = root.walk();
    walk_errors(&mut cursor, &source_code, &mut error_count);

    if error_count > 0 {
        if error_count > MAX_ERRORS {
            println!("... and {} more errors", error_count - MAX_ERRORS);
        }
        println!("\n✗ {} parse error(s) found", error_count);
        std::process::exit(1);
    } else {
        println!("✓ No parse errors!");
    }
}

fn walk_errors(cursor: &mut tree_sitter::TreeCursor, source: &str, error_count: &mut usize) {
    loop {
        let node = cursor.node();
        
        if node.is_error() || node.is_missing() || node.kind() == "invalid_prefixed_string_literal" {
            *error_count += 1;

            if *error_count <= MAX_ERRORS {
                let start = node.start_position();
                let end = node.end_position();
                let text = node.utf8_text(source.as_bytes()).unwrap_or("<invalid utf8>");

                if node.kind() == "invalid_prefixed_string_literal" {
                    println!(
                        "ERROR at {}:{}-{}:{}: invalid prefixed string literal \"{}\"",
                        start.row + 1, start.column + 1,
                        end.row + 1, end.column + 1,
                        text.chars().take(40).collect::<String>()
                    );
                } else if node.is_missing() {
                    println!(
                        "MISSING at {}:{}-{}:{}: expected {}",
                        start.row + 1, start.column + 1,
                        end.row + 1, end.column + 1,
                        node.kind()
                    );
                } else {
                    println!(
                        "ERROR at {}:{}-{}:{}: \"{}\"",
                        start.row + 1, start.column + 1,
                        end.row + 1, end.column + 1,
                        text.chars().take(40).collect::<String>()
                    );
                }
            }
        }

        // Recurse into children
        if cursor.goto_first_child() {
            walk_errors(cursor, source, error_count);
            cursor.goto_parent();
        }

        if !cursor.goto_next_sibling() {
            break;
        }
    }
}
