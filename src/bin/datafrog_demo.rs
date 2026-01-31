use std::error::Error;
use std::fs::File;
use std::path::PathBuf;

use datafrog::{Iteration, Relation, Variable};
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
        .ok_or("Usage: datafrog_demo <facts.json> [delta.json]")?;
    let delta_path = args.next().map(PathBuf::from);

    let base = read_tables(&PathBuf::from(base_path))?;

    let mut iteration = Iteration::new();
    let entity: Variable<(String, String)> = iteration.variable("entity");
    let uses_pkg: Variable<(String, String)> = iteration.variable("uses_pkg");
    let entity_uses_pkg: Variable<(String, String)> = iteration.variable("entity_uses_pkg");

    entity.insert(Relation::from_vec(
        base.entities
            .iter()
            .map(|e| (e.file.clone(), e.name.clone()))
            .collect(),
    ));
    uses_pkg.insert(Relation::from_vec(
        base.use_clauses
            .iter()
            .map(|u| (u.file.clone(), normalize_use_item(&u.item)))
            .collect(),
    ));

    while iteration.changed() {
        entity_uses_pkg.from_join(&entity, &uses_pkg, |_file, ent, pkg| {
            (ent.clone(), pkg.clone())
        });
    }

    println!(
        "base: entities={}, uses={}, derived={}",
        base.entities.len(),
        base.use_clauses.len(),
        variable_len(&entity_uses_pkg)
    );

    if let Some(delta_path) = delta_path {
        let delta = read_delta(&delta_path)?;
        if !delta.removed.entities.is_empty() || !delta.removed.use_clauses.is_empty() {
            eprintln!(
                "warning: removals present in delta; datafrog is monotonic so removals are ignored"
            );
        }

        if !delta.added.entities.is_empty() || !delta.added.use_clauses.is_empty() {
            entity.insert(Relation::from_vec(
                delta
                    .added
                    .entities
                    .iter()
                    .map(|e| (e.file.clone(), e.name.clone()))
                    .collect(),
            ));
            uses_pkg.insert(Relation::from_vec(
                delta
                    .added
                    .use_clauses
                    .iter()
                    .map(|u| (u.file.clone(), normalize_use_item(&u.item)))
                    .collect(),
            ));

            while iteration.changed() {
                entity_uses_pkg.from_join(&entity, &uses_pkg, |_file, ent, pkg| {
                    (ent.clone(), pkg.clone())
                });
            }

            println!(
                "delta applied: +entities={}, +uses={}, derived={}",
                delta.added.entities.len(),
                delta.added.use_clauses.len(),
                variable_len(&entity_uses_pkg)
            );
        }
    }

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

fn variable_len<T: Ord>(var: &Variable<T>) -> usize {
    let stable = var.stable.borrow();
    let stable_count: usize = stable.iter().map(|rel| rel.len()).sum();
    stable_count + var.recent.borrow().len()
}

#[cfg(test)]
mod tests {
    use super::normalize_use_item;

    #[test]
    fn strip_all_suffix() {
        assert_eq!(
            normalize_use_item("ieee.std_logic_1164.all"),
            "ieee.std_logic_1164"
        );
        assert_eq!(normalize_use_item("work.my_pkg"), "work.my_pkg");
    }
}
