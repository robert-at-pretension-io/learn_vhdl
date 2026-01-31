use std::cell::RefCell;
use std::error::Error;
use std::fs::File;
use std::path::PathBuf;
use std::rc::Rc;

use differential_dataflow::input::InputSession;
use differential_dataflow::operators::{Count, Join};
use serde::Deserialize;

#[derive(Debug, Deserialize, Default)]
struct Tables {
    #[serde(default)]
    entities: Vec<EntityRow>,
    #[serde(default)]
    use_clauses: Vec<UseClauseRow>,
}

#[derive(Debug, Deserialize, Default)]
struct Delta {
    #[serde(default)]
    added: Tables,
    #[serde(default)]
    removed: Tables,
}

#[derive(Debug, Deserialize)]
struct EntityRow {
    name: String,
    file: String,
}

#[derive(Debug, Deserialize)]
struct UseClauseRow {
    file: String,
    item: String,
}

fn main() -> Result<(), Box<dyn Error>> {
    let mut args = std::env::args().skip(1);
    let base_path = args
        .next()
        .ok_or("Usage: ddflow_demo <facts.json> [delta.json]")?;
    let delta_path = args.next().map(PathBuf::from);

    let base = read_tables(&PathBuf::from(base_path))?;
    let delta = if let Some(path) = delta_path {
        Some(read_delta(&path)?)
    } else {
        None
    };

    timely::execute_directly(move |worker| {
        let mut entities = InputSession::new();
        let mut uses = InputSession::new();

        let count_cell: Rc<RefCell<Option<usize>>> = Rc::new(RefCell::new(None));
        let count_cell_inner = count_cell.clone();

        let mut probe = timely::dataflow::operators::probe::Handle::new();

        worker.dataflow(|scope| {
            let entity = entities
                .to_collection(scope)
                .map(|(file, name)| (file, name));
            let uses_pkg = uses.to_collection(scope).map(|(file, pkg)| (file, pkg));

            let derived = entity
                .join(&uses_pkg)
                .map(|(_file, (ent, pkg))| ((), (ent, pkg)));

            derived
                .count()
                .inspect(move |((_, count), _, _)| {
                    *count_cell_inner.borrow_mut() = Some(*count as usize);
                })
                .probe_with(&mut probe);
        });

        for ent in &base.entities {
            entities.update((ent.file.clone(), ent.name.clone()), 1);
        }
        for use_row in &base.use_clauses {
            uses.update((use_row.file.clone(), normalize_use_item(&use_row.item)), 1);
        }
        entities.flush();
        uses.flush();

        entities.advance_to(1);
        uses.advance_to(1);
        entities.flush();
        uses.flush();
        while probe.less_than(entities.time()) {
            worker.step();
        }

        let base_count = count_cell.borrow().unwrap_or(0);
        println!(
            "base: entities={}, uses={}, derived={}",
            base.entities.len(),
            base.use_clauses.len(),
            base_count
        );

        if let Some(delta) = delta {
            for ent in &delta.added.entities {
                entities.update((ent.file.clone(), ent.name.clone()), 1);
            }
            for use_row in &delta.added.use_clauses {
                uses.update((use_row.file.clone(), normalize_use_item(&use_row.item)), 1);
            }

            for ent in &delta.removed.entities {
                entities.update((ent.file.clone(), ent.name.clone()), -1);
            }
            for use_row in &delta.removed.use_clauses {
                uses.update(
                    (use_row.file.clone(), normalize_use_item(&use_row.item)),
                    -1,
                );
            }
            entities.flush();
            uses.flush();

            entities.advance_to(2);
            uses.advance_to(2);
            entities.flush();
            uses.flush();
            while probe.less_than(entities.time()) {
                worker.step();
            }

            let next_count = count_cell.borrow().unwrap_or(0);
            println!(
                "delta applied: +entities={}, -entities={}, +uses={}, -uses={}, derived={}",
                delta.added.entities.len(),
                delta.removed.entities.len(),
                delta.added.use_clauses.len(),
                delta.removed.use_clauses.len(),
                next_count
            );
        }
    });

    Ok(())
}

fn read_tables(path: &PathBuf) -> Result<Tables, Box<dyn Error>> {
    let file = File::open(path)?;
    let tables: Tables = serde_json::from_reader(file)?;
    Ok(tables)
}

fn read_delta(path: &PathBuf) -> Result<Delta, Box<dyn Error>> {
    let file = File::open(path)?;
    let delta: Delta = serde_json::from_reader(file)?;
    Ok(delta)
}

fn normalize_use_item(item: &str) -> String {
    if let Some(prefix) = item.strip_suffix(".all") {
        prefix.to_string()
    } else {
        item.to_string()
    }
}
