use std::cell::RefCell;
use std::collections::HashMap;
use std::error::Error;
use std::io::{self, BufRead, Write};
use std::rc::Rc;

use differential_dataflow::input::InputSession;
use differential_dataflow::operators::{Consolidate, Join};
use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize, Default, Clone)]
struct Tables {
    #[serde(default)]
    entities: Vec<EntityRow>,
    #[serde(default)]
    architectures: Vec<ArchitectureRow>,
    #[serde(default)]
    ports: Vec<PortRow>,
    #[serde(default)]
    dependencies: Vec<DependencyRow>,
    #[serde(default)]
    symbols: Vec<SymbolRow>,
}

#[derive(Debug, Deserialize)]
struct Command {
    kind: String,
    #[serde(default)]
    tables: Tables,
    #[serde(default)]
    added: Tables,
    #[serde(default)]
    removed: Tables,
}

#[derive(Debug, Deserialize, Clone)]
struct EntityRow {
    name: String,
    file: String,
    line: i64,
}

#[derive(Debug, Deserialize, Clone)]
struct ArchitectureRow {
    name: String,
    entity_name: String,
    file: String,
    line: i64,
}

#[derive(Debug, Deserialize, Clone)]
struct PortRow {
    entity: String,
    name: String,
    direction: String,
    file: String,
    line: i64,
}

#[derive(Debug, Deserialize, Clone)]
struct DependencyRow {
    file: String,
    target: String,
    kind: String,
    line: i64,
}

#[derive(Debug, Deserialize, Clone)]
struct SymbolRow {
    name: String,
}

#[derive(Debug, Clone, Eq, PartialEq, Hash)]
struct ViolationKey {
    rule: String,
    severity: String,
    file: String,
    line: i64,
    message: String,
}

#[derive(Debug, Serialize, Clone)]
struct Violation {
    rule: String,
    severity: String,
    file: String,
    line: i64,
    message: String,
}

#[derive(Debug, Serialize, Default)]
struct Summary {
    total_violations: usize,
    errors: usize,
    warnings: usize,
    info: usize,
}

#[derive(Debug, Serialize)]
struct Response {
    kind: String,
    summary: Summary,
    violations: Vec<Violation>,
}

fn main() -> Result<(), Box<dyn Error>> {
    let (tx, rx) = std::sync::mpsc::channel::<String>();
    let rx = std::sync::Arc::new(std::sync::Mutex::new(rx));
    std::thread::spawn(move || {
        let stdin = io::stdin();
        for line in stdin.lock().lines() {
            match line {
                Ok(line) => {
                    if tx.send(line).is_err() {
                        break;
                    }
                }
                Err(_) => break,
            }
        }
    });

    let mut epoch: u64 = 1;

    timely::execute_directly(move |worker| {
        let rx = rx.clone();
        let mut stdout = io::BufWriter::new(io::stdout());
        let mut entities: Session<(String, String, i64)> = InputSession::new();
        let mut architectures: Session<(String, String, i64, String)> = InputSession::new();
        let mut ports: Session<(String, String)> = InputSession::new();
        let mut dependencies: Session<(String, String, i64, String)> = InputSession::new();
        let mut symbols: Session<String> = InputSession::new();

        let violations_state: Rc<RefCell<HashMap<ViolationKey, isize>>> =
            Rc::new(RefCell::new(HashMap::new()));
        let violations_state_inner = violations_state.clone();

        let mut probe = timely::dataflow::operators::probe::Handle::new();

        worker.dataflow(|scope| {
            let entity_rows = entities.to_collection(scope).map(|(name, file, line)| {
                let name_clone = name.clone();
                (name, (file, line, name_clone))
            });
            let arch_rows = architectures
                .to_collection(scope)
                .map(|(entity, file, line, name)| (entity, (file, line, name)));
            let port_entities = ports
                .to_collection(scope)
                .map(|(entity, _name)| (entity, ()));
            let dep_rows = dependencies
                .to_collection(scope)
                .map(|(target, file, line, kind)| {
                    let target_clone = target.clone();
                    (target, (file, line, kind, target_clone))
                });
            let sym_rows = symbols.to_collection(scope).map(|name| (name, ()));

            let entity_ports = entity_rows.join_map(&port_entities, |entity, payload, _| {
                (entity.clone(), payload.clone())
            });
            let entities_without_ports = entity_rows
                .concat(&entity_ports.negate())
                .consolidate()
                .filter(|(_entity, (_file, _line, name))| !is_testbench_name(name))
                .map(|(_entity, (file, line, name))| ViolationKey {
                    rule: "entity_has_ports".to_string(),
                    severity: "warning".to_string(),
                    file,
                    line,
                    message: format!("Entity '{}' has no ports defined", name),
                });

            let arch_with_entity = arch_rows.join_map(&entity_rows, |entity, payload, _| {
                (entity.clone(), payload.clone())
            });
            let orphan_arch = arch_rows
                .concat(&arch_with_entity.negate())
                .consolidate()
                .map(|(entity, (file, line, name))| ViolationKey {
                    rule: "architecture_has_entity".to_string(),
                    severity: "error".to_string(),
                    file,
                    line,
                    message: format!(
                        "Architecture '{}' references undefined entity '{}'",
                        name, entity
                    ),
                });

            let entity_with_arch = entity_rows.join_map(&arch_rows, |entity, payload, _| {
                (entity.clone(), payload.clone())
            });
            let entities_without_arch = entity_rows
                .concat(&entity_with_arch.negate())
                .consolidate()
                .map(|(_entity, (file, line, name))| ViolationKey {
                    rule: "entity_without_arch".to_string(),
                    severity: "warning".to_string(),
                    file,
                    line,
                    message: format!("Entity '{}' has no architecture defined", name),
                });

            let dep_symbols = dep_rows.join_map(&sym_rows, |target, payload, _| {
                (target.clone(), payload.clone())
            });
            let unresolved = dep_rows
                .filter(|(_target, (_file, _line, kind, _t))| kind == "instantiation")
                .concat(&dep_symbols.negate())
                .consolidate()
                .map(|(_target, (file, line, _kind, dep_target))| ViolationKey {
                    rule: "unresolved_dependency".to_string(),
                    severity: "error".to_string(),
                    file,
                    line,
                    message: format!("Unresolved dependency: '{}'", dep_target),
                });

            let all = entities_without_ports
                .concat(&orphan_arch)
                .concat(&entities_without_arch)
                .concat(&unresolved);

            all.inspect(move |(violation, _time, diff)| {
                let mut map = violations_state_inner.borrow_mut();
                let entry = map.entry(violation.clone()).or_insert(0);
                *entry += diff;
                if *entry == 0 {
                    map.remove(violation);
                }
            })
            .probe_with(&mut probe);
        });

        loop {
            let line = {
                let guard = rx.lock().expect("rx mutex poisoned");
                guard.recv()
            };
            let line = match line {
                Ok(line) => line,
                Err(_) => break,
            };

            if line.trim().is_empty() {
                continue;
            }
            let cmd: Command = match serde_json::from_str(&line) {
                Ok(cmd) => cmd,
                Err(err) => {
                    let _ = writeln!(
                        stdout,
                        "{{\"kind\":\"error\",\"message\":\"invalid command: {}\"}}",
                        err
                    );
                    let _ = stdout.flush();
                    continue;
                }
            };

            match cmd.kind.as_str() {
                "init" => {
                    apply_tables(
                        &cmd.tables,
                        &mut entities,
                        &mut architectures,
                        &mut ports,
                        &mut dependencies,
                        &mut symbols,
                        1,
                    );
                }
                "delta" => {
                    apply_tables(
                        &cmd.added,
                        &mut entities,
                        &mut architectures,
                        &mut ports,
                        &mut dependencies,
                        &mut symbols,
                        1,
                    );
                    apply_tables(
                        &cmd.removed,
                        &mut entities,
                        &mut architectures,
                        &mut ports,
                        &mut dependencies,
                        &mut symbols,
                        -1,
                    );
                }
                "snapshot" => {}
                _ => {
                    let _ = writeln!(
                        stdout,
                        "{{\"kind\":\"error\",\"message\":\"unknown command kind: {}\"}}",
                        cmd.kind
                    );
                    let _ = stdout.flush();
                    continue;
                }
            }

            entities.advance_to(epoch);
            architectures.advance_to(epoch);
            ports.advance_to(epoch);
            dependencies.advance_to(epoch);
            symbols.advance_to(epoch);
            entities.flush();
            architectures.flush();
            ports.flush();
            dependencies.flush();
            symbols.flush();

            while probe.less_than(entities.time()) {
                worker.step();
            }

            let response = build_response(&violations_state.borrow());
            let payload = serde_json::to_string(&response).unwrap_or_else(|_| {
                "{\"kind\":\"error\",\"message\":\"failed to serialize response\"}".to_string()
            });
            let _ = writeln!(stdout, "{}", payload);
            let _ = stdout.flush();

            epoch += 1;
        }
    });

    Ok(())
}

fn apply_tables(
    tables: &Tables,
    entities: &mut Session<(String, String, i64)>,
    architectures: &mut Session<(String, String, i64, String)>,
    ports: &mut Session<(String, String)>,
    dependencies: &mut Session<(String, String, i64, String)>,
    symbols: &mut Session<String>,
    weight: isize,
) {
    for ent in &tables.entities {
        entities.update((ent.name.clone(), ent.file.clone(), ent.line), weight);
    }
    for arch in &tables.architectures {
        architectures.update(
            (
                arch.entity_name.clone(),
                arch.file.clone(),
                arch.line,
                arch.name.clone(),
            ),
            weight,
        );
    }
    for port in &tables.ports {
        ports.update((port.entity.clone(), port.name.clone()), weight);
    }
    for dep in &tables.dependencies {
        dependencies.update(
            (
                dep.target.clone(),
                dep.file.clone(),
                dep.line,
                dep.kind.clone(),
            ),
            weight,
        );
    }
    for sym in &tables.symbols {
        symbols.update(sym.name.clone(), weight);
    }
}

fn build_response(violations: &HashMap<ViolationKey, isize>) -> Response {
    let mut list: Vec<Violation> = violations
        .iter()
        .filter(|(_key, weight)| **weight > 0)
        .map(|(key, _)| Violation {
            rule: key.rule.clone(),
            severity: key.severity.clone(),
            file: key.file.clone(),
            line: key.line,
            message: key.message.clone(),
        })
        .collect();
    list.sort_by(|a, b| a.file.cmp(&b.file).then(a.line.cmp(&b.line)));

    let mut summary = Summary::default();
    summary.total_violations = list.len();
    for v in &list {
        match v.severity.as_str() {
            "error" => summary.errors += 1,
            "warning" => summary.warnings += 1,
            "info" => summary.info += 1,
            _ => {}
        }
    }

    Response {
        kind: "snapshot".to_string(),
        summary,
        violations: list,
    }
}

fn is_testbench_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.contains("_tb")
        || lower.contains("tb_")
        || lower.ends_with("tb")
        || lower.contains("test")
        || lower.contains("bench")
}

type Session<D> = InputSession<u64, D, isize>;
