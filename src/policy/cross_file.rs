use crate::policy::helpers;
use crate::policy::input::{
    Architecture, Entity, FunctionDeclaration, GenericDecl, Input, Instance, Port,
    ProcedureDeclaration, Process, RecordField, SubprogramParameter, TypeDeclaration,
};
use crate::policy::result::Violation;
use std::collections::{HashMap, HashSet};

pub fn violations(input: &Input) -> Vec<Violation> {
    let file_libs = file_library_map(input);
    let instantiations = collect_package_instantiations(input, &file_libs);
    let visibility = build_visibility(input);
    let resolver = TypeResolver::new(input, &visibility, &instantiations);
    let bindings = build_binding_table(input, &visibility);

    let mut out = Vec::new();
    out.extend(duplicate_architecture_in_library(input, &file_libs));
    out.extend(unresolved_type_marks(input, &resolver));
    out.extend(package_body_without_declaration(input, &file_libs));
    out.extend(instance_binding_violations(input, &bindings));
    out.extend(port_map_conformance(input, &resolver, &bindings));
    out.extend(generic_map_conformance(input, &resolver, &bindings));
    out.extend(subprogram_resolution(
        input,
        &resolver,
        &visibility,
        &instantiations,
    ));
    out.extend(configuration_checks(input));
    out.extend(cross_file_assignment_checks(input, &resolver, &bindings));
    out
}

#[derive(Default, Clone)]
struct Visibility {
    library: String,
    visible_packages: HashSet<String>,
    visible_libraries: HashSet<String>,
}

#[derive(Clone)]
struct PackageInstantiation {
    inst_lib: String,
    inst_pkg: String,
    target_lib: String,
    target_pkg: String,
}

fn collect_package_instantiations(
    input: &Input,
    file_libs: &HashMap<String, String>,
) -> Vec<PackageInstantiation> {
    let mut by_file_line: HashMap<(String, usize), String> = HashMap::new();
    for pkg in &input.packages {
        if pkg.name.trim().is_empty() {
            continue;
        }
        by_file_line.insert((pkg.file.clone(), pkg.line), pkg.name.to_ascii_lowercase());
    }

    let mut out = Vec::new();
    for dep in &input.dependencies {
        if !dep.kind.eq_ignore_ascii_case("package_instantiation") {
            continue;
        }
        let inst_pkg = match by_file_line.get(&(dep.source.clone(), dep.line)) {
            Some(name) => name.clone(),
            None => continue,
        };
        let inst_lib = file_libs
            .get(&dep.source)
            .cloned()
            .unwrap_or_else(|| "work".to_string())
            .to_ascii_lowercase();
        let (lib_opt, target_name) = split_target(&dep.target);
        let mut target_lib = lib_opt.unwrap_or_else(|| inst_lib.clone());
        if target_lib.eq_ignore_ascii_case("work") {
            target_lib = inst_lib.clone();
        }
        if target_name.trim().is_empty() {
            continue;
        }
        out.push(PackageInstantiation {
            inst_lib,
            inst_pkg,
            target_lib: target_lib.to_ascii_lowercase(),
            target_pkg: target_name.to_ascii_lowercase(),
        });
    }

    out
}

fn build_visibility(input: &Input) -> HashMap<String, Visibility> {
    let file_libs = file_library_map(input);
    let mut context_map: HashMap<String, ContextInfo> = HashMap::new();

    for ctx in &input.context_declarations {
        let lib = file_libs
            .get(&ctx.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string());
        let key = format!(
            "{}.{}",
            lib.to_ascii_lowercase(),
            ctx.name.to_ascii_lowercase()
        );
        let info = ContextInfo {
            libraries: ctx
                .libraries
                .iter()
                .map(|s| s.to_ascii_lowercase())
                .collect(),
            use_items: ctx.use_items.clone(),
            context_refs: ctx.context_refs.clone(),
        };
        context_map.insert(key.clone(), info.clone());
        context_map
            .entry(ctx.name.to_ascii_lowercase())
            .or_insert(info);
    }

    let mut visibility = HashMap::new();

    for file in &input.files {
        let lib = file.library.to_ascii_lowercase();
        let mut vis = Visibility::default();
        vis.library = lib.clone();
        vis.visible_libraries = ["work", "ieee", "std"]
            .iter()
            .map(|s| s.to_string())
            .collect();
        if !lib.is_empty() {
            vis.visible_libraries.insert(lib.clone());
        }

        for clause in &input.library_clauses {
            if clause.file != file.path {
                continue;
            }
            for l in &clause.libraries {
                vis.visible_libraries.insert(l.to_ascii_lowercase());
            }
        }

        for ctx in &input.context_clauses {
            if ctx.file != file.path {
                continue;
            }
            apply_context(&ctx.name, &mut vis, &context_map);
        }

        for use_clause in &input.use_clauses {
            if use_clause.file != file.path {
                continue;
            }
            for item in &use_clause.items {
                if let Some(pkg_key) = parse_use_item(item, &vis) {
                    vis.visible_packages.insert(pkg_key);
                }
            }
        }

        visibility.insert(file.path.clone(), vis);
    }

    let mut entity_files: HashMap<(String, String), Vec<String>> = HashMap::new();
    for ent in &input.entities {
        let lib = file_libs
            .get(&ent.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string());
        entity_files
            .entry((lib.to_ascii_lowercase(), ent.name.to_ascii_lowercase()))
            .or_default()
            .push(ent.file.clone());
    }

    for arch in &input.architectures {
        let lib = file_libs
            .get(&arch.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string())
            .to_ascii_lowercase();
        let key = (lib, arch.entity_name.to_ascii_lowercase());
        let ent_files = match entity_files.get(&key) {
            Some(files) => files,
            None => continue,
        };
        let mut ent_libraries = HashSet::new();
        let mut ent_packages = HashSet::new();
        for ent_file in ent_files {
            if let Some(ent_vis) = visibility.get(ent_file) {
                ent_libraries.extend(ent_vis.visible_libraries.iter().cloned());
                ent_packages.extend(ent_vis.visible_packages.iter().cloned());
            }
        }
        if let Some(vis) = visibility.get_mut(&arch.file) {
            vis.visible_libraries.extend(ent_libraries);
            vis.visible_packages.extend(ent_packages);
        }
    }

    visibility
}

#[derive(Clone)]
struct ContextInfo {
    libraries: Vec<String>,
    use_items: Vec<String>,
    context_refs: Vec<String>,
}

fn apply_context(name: &str, vis: &mut Visibility, contexts: &HashMap<String, ContextInfo>) {
    let mut visited = HashSet::new();
    apply_context_inner(name, vis, contexts, &mut visited);
}

fn apply_context_inner(
    name: &str,
    vis: &mut Visibility,
    contexts: &HashMap<String, ContextInfo>,
    visited: &mut HashSet<String>,
) {
    let key = name.to_ascii_lowercase();
    if !visited.insert(key.clone()) {
        return;
    }
    let ctx = match contexts.get(&key) {
        Some(c) => c,
        None => return,
    };

    for lib in &ctx.libraries {
        vis.visible_libraries.insert(lib.to_ascii_lowercase());
    }
    for item in &ctx.use_items {
        if let Some(pkg_key) = parse_use_item(item, vis) {
            vis.visible_packages.insert(pkg_key);
        }
    }
    for cref in &ctx.context_refs {
        apply_context_inner(cref, vis, contexts, visited);
    }
}

fn parse_use_item(item: &str, vis: &Visibility) -> Option<String> {
    let raw = item.trim();
    if raw.is_empty() {
        return None;
    }
    let parts: Vec<&str> = raw.split('.').collect();
    if parts.is_empty() {
        return None;
    }

    let lib = if parts.len() >= 3 {
        parts[0]
    } else if parts.len() == 2 {
        if vis
            .visible_libraries
            .contains(&parts[0].to_ascii_lowercase())
        {
            parts[0]
        } else {
            vis.library.as_str()
        }
    } else {
        vis.library.as_str()
    };

    let pkg = if parts.len() >= 3 {
        parts[1]
    } else if parts.len() == 2 {
        if vis
            .visible_libraries
            .contains(&parts[0].to_ascii_lowercase())
        {
            parts[1]
        } else {
            parts[0]
        }
    } else {
        parts[0]
    };

    let mut lib_name = lib.to_ascii_lowercase();
    if lib_name == "work" {
        lib_name = vis.library.to_ascii_lowercase();
    }

    if lib_name.is_empty() || pkg.is_empty() {
        return None;
    }
    Some(format!("{}.{}", lib_name, pkg.to_ascii_lowercase()))
}

struct TypeResolver {
    visibility: HashMap<String, Visibility>,
    physical_types: HashSet<String>,
    pkg_types: HashMap<String, HashMap<String, TypeDeclaration>>,
    pkg_subtypes: HashMap<String, HashMap<String, SubtypeInfo>>,
    arch_types: HashMap<(String, String), HashMap<String, TypeDeclaration>>,
    arch_subtypes: HashMap<(String, String), HashMap<String, SubtypeInfo>>,
    pkg_constants: HashMap<String, HashMap<String, String>>,
    arch_constants: HashMap<(String, String), HashMap<String, String>>,
    pkg_enum_literals: HashMap<String, HashMap<String, String>>,
    arch_enum_literals: HashMap<(String, String), HashMap<String, String>>,
    subprogram_decls: Vec<SubprogramDecl>,
}

impl TypeResolver {
    fn new(
        input: &Input,
        visibility: &HashMap<String, Visibility>,
        instantiations: &[PackageInstantiation],
    ) -> Self {
        let file_libs = file_library_map(input);
        let subprogram_decls = build_subprogram_decls(input, instantiations);
        let mut physical_types = HashSet::new();
        let mut pkg_types: HashMap<String, HashMap<String, TypeDeclaration>> = HashMap::new();
        let mut pkg_subtypes: HashMap<String, HashMap<String, SubtypeInfo>> = HashMap::new();
        let mut arch_types: HashMap<(String, String), HashMap<String, TypeDeclaration>> =
            HashMap::new();
        let mut arch_subtypes: HashMap<(String, String), HashMap<String, SubtypeInfo>> =
            HashMap::new();
        let mut pkg_constants: HashMap<String, HashMap<String, String>> = HashMap::new();
        let mut arch_constants: HashMap<(String, String), HashMap<String, String>> = HashMap::new();
        let mut pkg_enum_literals: HashMap<String, HashMap<String, String>> = HashMap::new();
        let mut arch_enum_literals: HashMap<(String, String), HashMap<String, String>> =
            HashMap::new();

        for ty in &input.types {
            if ty.kind.eq_ignore_ascii_case("physical") {
                let name = ty.name.to_ascii_lowercase();
                if !name.is_empty() {
                    physical_types.insert(name.clone());
                }
                if !ty.in_package.is_empty() {
                    let lib = file_libs
                        .get(&ty.file)
                        .cloned()
                        .unwrap_or_else(|| "work".to_string())
                        .to_ascii_lowercase();
                    physical_types.insert(format!(
                        "{}.{}.{}",
                        lib,
                        ty.in_package.to_ascii_lowercase(),
                        name
                    ));
                }
            }
            if !ty.in_package.is_empty() {
                let lib = file_libs
                    .get(&ty.file)
                    .cloned()
                    .unwrap_or_else(|| "work".to_string());
                let key = format!(
                    "{}.{}",
                    lib.to_ascii_lowercase(),
                    ty.in_package.to_ascii_lowercase()
                );
                pkg_types
                    .entry(key)
                    .or_default()
                    .insert(ty.name.to_ascii_lowercase(), ty.clone());
            } else if !ty.in_arch.is_empty() {
                let key = (ty.file.clone(), ty.in_arch.to_ascii_lowercase());
                arch_types
                    .entry(key)
                    .or_default()
                    .insert(ty.name.to_ascii_lowercase(), ty.clone());
            }
        }

        for st in &input.subtypes {
            if !st.in_package.is_empty() {
                let lib = file_libs
                    .get(&st.file)
                    .cloned()
                    .unwrap_or_else(|| "work".to_string());
                let key = format!(
                    "{}.{}",
                    lib.to_ascii_lowercase(),
                    st.in_package.to_ascii_lowercase()
                );
                pkg_subtypes.entry(key).or_default().insert(
                    st.name.to_ascii_lowercase(),
                    SubtypeInfo {
                        base: st.base_type.clone(),
                        constraint: st.constraint.clone(),
                    },
                );
            } else if !st.in_arch.is_empty() {
                let key = (st.file.clone(), st.in_arch.to_ascii_lowercase());
                arch_subtypes.entry(key).or_default().insert(
                    st.name.to_ascii_lowercase(),
                    SubtypeInfo {
                        base: st.base_type.clone(),
                        constraint: st.constraint.clone(),
                    },
                );
            }
        }

        for c in &input.constant_decls {
            if c.name.trim().is_empty() || c.r#type.trim().is_empty() {
                continue;
            }
            if !c.in_package.is_empty() {
                let lib = file_libs
                    .get(&c.file)
                    .cloned()
                    .unwrap_or_else(|| "work".to_string());
                let key = format!(
                    "{}.{}",
                    lib.to_ascii_lowercase(),
                    c.in_package.to_ascii_lowercase()
                );
                pkg_constants
                    .entry(key)
                    .or_default()
                    .insert(c.name.to_ascii_lowercase(), c.r#type.clone());
            } else if !c.in_arch.is_empty() {
                let key = (c.file.clone(), c.in_arch.to_ascii_lowercase());
                arch_constants
                    .entry(key)
                    .or_default()
                    .insert(c.name.to_ascii_lowercase(), c.r#type.clone());
            }
        }

        for ty in &input.types {
            if !ty.kind.eq_ignore_ascii_case("enum") || ty.enum_literals.is_empty() {
                continue;
            }
            if !ty.in_package.is_empty() {
                let lib = file_libs
                    .get(&ty.file)
                    .cloned()
                    .unwrap_or_else(|| "work".to_string());
                let key = format!(
                    "{}.{}",
                    lib.to_ascii_lowercase(),
                    ty.in_package.to_ascii_lowercase()
                );
                let entry = pkg_enum_literals.entry(key).or_default();
                for lit in &ty.enum_literals {
                    if !lit.trim().is_empty() {
                        entry.insert(lit.to_ascii_lowercase(), ty.name.clone());
                    }
                }
            } else if !ty.in_arch.is_empty() {
                let key = (ty.file.clone(), ty.in_arch.to_ascii_lowercase());
                let entry = arch_enum_literals.entry(key).or_default();
                for lit in &ty.enum_literals {
                    if !lit.trim().is_empty() {
                        entry.insert(lit.to_ascii_lowercase(), ty.name.clone());
                    }
                }
            }
        }

        for inst in instantiations {
            let target_key = format!("{}.{}", inst.target_lib, inst.target_pkg);
            let inst_key = format!("{}.{}", inst.inst_lib, inst.inst_pkg);
            if let Some(types) = pkg_types.get(&target_key).cloned() {
                let entry = pkg_types.entry(inst_key.clone()).or_default();
                for (name, ty) in types {
                    entry.entry(name).or_insert(ty);
                }
            }
            if let Some(subtypes) = pkg_subtypes.get(&target_key).cloned() {
                let entry = pkg_subtypes.entry(inst_key).or_default();
                for (name, st) in subtypes {
                    entry.entry(name).or_insert(st);
                }
            }
        }

        Self {
            visibility: visibility.clone(),
            physical_types,
            pkg_types,
            pkg_subtypes,
            arch_types,
            arch_subtypes,
            pkg_constants,
            arch_constants,
            pkg_enum_literals,
            arch_enum_literals,
            subprogram_decls,
        }
    }

    fn resolve_type(&self, file: &str, arch: &str, raw_type: &str) -> Option<ResolvedType> {
        let base = strip_type_mark(raw_type);
        if base.is_empty() {
            return None;
        }
        let lower = base.to_ascii_lowercase();
        if is_builtin_type(&lower) {
            return Some(ResolvedType::builtin(lower));
        }

        let (lib_opt, pkg_opt, name) = split_type_reference(&lower);

        if let (Some(lib), Some(pkg)) = (lib_opt.as_ref(), pkg_opt.as_ref()) {
            return self.resolve_in_package(lib, pkg, &name, file, arch);
        }

        if let Some(pkg) = pkg_opt.as_ref() {
            if let Some(lib) = self.find_visible_package_lib(file, pkg) {
                return self.resolve_in_package(&lib, pkg, &name, file, arch);
            }
            return None;
        }

        if let Some(ty) = self.resolve_in_arch(file, arch, &name) {
            return Some(ty);
        }

        let candidates = self.resolve_in_visible_packages(file, arch, &name);
        if candidates.len() == 1 {
            return Some(candidates[0].clone());
        }

        None
    }

    fn resolve_in_package(
        &self,
        lib: &str,
        pkg: &str,
        name: &str,
        file: &str,
        arch: &str,
    ) -> Option<ResolvedType> {
        let key = format!("{}.{}", lib, pkg);
        if let Some(map) = self.pkg_types.get(&key) {
            if let Some(ty) = map.get(name) {
                return Some(ResolvedType::from_decl(lib, pkg, ty));
            }
        }
        if let Some(map) = self.pkg_subtypes.get(&key) {
            if let Some(info) = map.get(name) {
                if let Some(resolved) = self.resolve_type(file, arch, &info.base) {
                    return Some(resolved.with_constraint(&info.constraint));
                }
            }
        }
        None
    }

    fn resolve_in_arch(&self, file: &str, arch: &str, name: &str) -> Option<ResolvedType> {
        if arch.is_empty() {
            return None;
        }
        let base_arch = base_arch_name(arch);
        let arch_keys = if base_arch != arch {
            vec![arch.to_string(), base_arch.to_string()]
        } else {
            vec![arch.to_string()]
        };
        for arch_name in arch_keys {
            let key = (file.to_string(), arch_name.to_ascii_lowercase());
            if let Some(map) = self.arch_types.get(&key) {
                if let Some(ty) = map.get(name) {
                    return Some(ResolvedType::from_decl("", "", ty));
                }
            }
            if let Some(map) = self.arch_subtypes.get(&key) {
                if let Some(info) = map.get(name) {
                    if let Some(resolved) = self.resolve_type(file, &arch_name, &info.base) {
                        return Some(resolved.with_constraint(&info.constraint));
                    }
                }
            }
        }
        None
    }

    fn resolve_constant_type(&self, file: &str, arch: &str, name: &str) -> Option<ResolvedType> {
        if name.trim().is_empty() {
            return None;
        }
        let lower = name.to_ascii_lowercase();
        if !arch.is_empty() {
            let base_arch = base_arch_name(arch);
            let arch_keys = if base_arch != arch {
                vec![arch.to_string(), base_arch.to_string()]
            } else {
                vec![arch.to_string()]
            };
            for arch_name in arch_keys {
                let key = (file.to_string(), arch_name.to_ascii_lowercase());
                if let Some(map) = self.arch_constants.get(&key) {
                    if let Some(ty) = map.get(&lower) {
                        return self.resolve_type(file, &arch_name, ty);
                    }
                }
            }
        }
        let vis = self.visibility.get(file)?;
        let mut hits: Vec<(String, String)> = Vec::new();
        for pkg_key in &vis.visible_packages {
            if let Some(map) = self.pkg_constants.get(pkg_key) {
                if let Some(ty) = map.get(&lower) {
                    hits.push((pkg_key.clone(), ty.clone()));
                }
            }
        }
        if hits.len() != 1 {
            return None;
        }
        let (pkg_key, ty) = &hits[0];
        let parts: Vec<&str> = pkg_key.split('.').collect();
        if parts.len() >= 2 {
            if let Some(resolved) =
                self.resolve_in_package(parts[0], parts[1], &strip_type_mark(ty), file, arch)
            {
                return Some(resolved);
            }
        }
        self.resolve_type(file, arch, ty)
    }

    fn resolve_enum_literal_type(
        &self,
        file: &str,
        arch: &str,
        name: &str,
    ) -> Option<ResolvedType> {
        if name.trim().is_empty() {
            return None;
        }
        let lower = name.to_ascii_lowercase();
        if !arch.is_empty() {
            let base_arch = base_arch_name(arch);
            let arch_keys = if base_arch != arch {
                vec![arch.to_string(), base_arch.to_string()]
            } else {
                vec![arch.to_string()]
            };
            for arch_name in arch_keys {
                let key = (file.to_string(), arch_name.to_ascii_lowercase());
                if let Some(map) = self.arch_enum_literals.get(&key) {
                    if let Some(ty) = map.get(&lower) {
                        return self.resolve_type(file, &arch_name, ty);
                    }
                }
            }
        }
        let vis = self.visibility.get(file)?;
        let mut hits: Vec<(String, String)> = Vec::new();
        for pkg_key in &vis.visible_packages {
            if let Some(map) = self.pkg_enum_literals.get(pkg_key) {
                if let Some(ty) = map.get(&lower) {
                    hits.push((pkg_key.clone(), ty.clone()));
                }
            }
        }
        if hits.len() != 1 {
            return None;
        }
        let (pkg_key, ty) = &hits[0];
        let parts: Vec<&str> = pkg_key.split('.').collect();
        if parts.len() >= 2 {
            if let Some(resolved) =
                self.resolve_in_package(parts[0], parts[1], &strip_type_mark(ty), file, arch)
            {
                return Some(resolved);
            }
        }
        self.resolve_type(file, arch, ty)
    }

    fn resolve_qualified_constant_or_enum(
        &self,
        file: &str,
        arch: &str,
        raw: &str,
    ) -> Option<ResolvedType> {
        let trimmed = raw.trim();
        if trimmed.is_empty() {
            return None;
        }
        let parts: Vec<&str> = trimmed.split('.').collect();
        if parts.len() < 2 {
            return None;
        }
        let (lib, pkg, name) = if parts.len() >= 3 {
            (
                parts[0].to_string(),
                parts[1].to_string(),
                parts[parts.len() - 1].to_string(),
            )
        } else {
            let vis = self.visibility.get(file)?;
            let pkg = parts[0].to_string();
            let name = parts[1].to_string();
            let lib = self.find_visible_package_lib(file, &pkg)?;
            let key = format!("{}.{}", lib.to_ascii_lowercase(), pkg.to_ascii_lowercase());
            if !vis.visible_packages.contains(&key) {
                return None;
            }
            (lib, pkg, name)
        };
        let key = format!("{}.{}", lib.to_ascii_lowercase(), pkg.to_ascii_lowercase());
        if let Some(map) = self.pkg_constants.get(&key) {
            if let Some(ty) = map.get(&name.to_ascii_lowercase()) {
                if let Some(resolved) =
                    self.resolve_in_package(&lib, &pkg, &strip_type_mark(ty), file, arch)
                {
                    return Some(resolved);
                }
                return self.resolve_type(file, arch, ty);
            }
        }
        if let Some(map) = self.pkg_enum_literals.get(&key) {
            if let Some(ty) = map.get(&name.to_ascii_lowercase()) {
                if let Some(resolved) =
                    self.resolve_in_package(&lib, &pkg, &strip_type_mark(ty), file, arch)
                {
                    return Some(resolved);
                }
                return self.resolve_type(file, arch, ty);
            }
        }
        None
    }

    fn resolve_in_visible_packages(&self, file: &str, arch: &str, name: &str) -> Vec<ResolvedType> {
        let mut out = Vec::new();
        let vis = match self.visibility.get(file) {
            Some(v) => v,
            None => return out,
        };
        for pkg_key in &vis.visible_packages {
            if let Some(map) = self.pkg_types.get(pkg_key) {
                if let Some(ty) = map.get(name) {
                    let parts: Vec<&str> = pkg_key.split('.').collect();
                    let lib = parts.get(0).cloned().unwrap_or("");
                    let pkg = parts.get(1).cloned().unwrap_or("");
                    out.push(ResolvedType::from_decl(lib, pkg, ty));
                }
            }
            if let Some(map) = self.pkg_subtypes.get(pkg_key) {
                if let Some(info) = map.get(name) {
                    if let Some(resolved) = self.resolve_type(file, arch, &info.base) {
                        out.push(resolved.with_constraint(&info.constraint));
                    }
                }
            }
        }
        out
    }

    fn find_visible_package_lib(&self, file: &str, pkg: &str) -> Option<String> {
        let vis = self.visibility.get(file)?;
        let mut hits = Vec::new();
        for pkg_key in &vis.visible_packages {
            let parts: Vec<&str> = pkg_key.split('.').collect();
            if parts.len() < 2 {
                continue;
            }
            if parts[1].eq_ignore_ascii_case(pkg) {
                hits.push(parts[0].to_string());
            }
        }
        if hits.len() == 1 {
            return Some(hits[0].clone());
        }
        None
    }

    fn is_physical_type_name(&self, name: &str) -> bool {
        let key = name.to_ascii_lowercase();
        self.physical_types.contains(&key)
    }
}

#[derive(Clone)]
struct ResolvedType {
    canonical: String,
    kind: String,
    constraint: Option<String>,
    element_type: Option<String>,
    fields: Vec<RecordField>,
}

impl ResolvedType {
    fn builtin(name: String) -> Self {
        let kind = if is_numeric_type(&name) {
            "numeric".to_string()
        } else {
            name.clone()
        };
        Self {
            canonical: name,
            kind,
            constraint: None,
            element_type: None,
            fields: Vec::new(),
        }
    }

    fn from_decl(lib: &str, pkg: &str, decl: &TypeDeclaration) -> Self {
        let mut canonical = decl.name.to_ascii_lowercase();
        if !pkg.is_empty() {
            canonical = format!("{}.{}.{}", lib, pkg, canonical);
        }
        let element_type =
            if decl.kind.eq_ignore_ascii_case("array") && !decl.element_type.trim().is_empty() {
                Some(decl.element_type.clone())
            } else {
                None
            };
        let fields = if decl.kind.eq_ignore_ascii_case("record") {
            decl.fields.clone()
        } else {
            Vec::new()
        };
        Self {
            canonical,
            kind: decl.kind.to_ascii_lowercase(),
            constraint: None,
            element_type,
            fields,
        }
    }

    fn with_constraint(mut self, constraint: &str) -> Self {
        let trimmed = constraint.trim();
        if !trimmed.is_empty() {
            self.constraint = Some(normalize_constraint(trimmed));
        }
        self
    }
}

#[derive(Clone)]
struct SubtypeInfo {
    base: String,
    constraint: String,
}

fn strip_type_mark(raw: &str) -> String {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return String::new();
    }
    let mut base = trimmed;
    if let Some(idx) = base.find('(') {
        base = &base[..idx];
    }
    if let Some(idx) = base.to_ascii_lowercase().find(" range ") {
        base = &base[..idx];
    }
    base.split_whitespace()
        .next()
        .unwrap_or("")
        .trim()
        .to_string()
}

fn split_type_reference(name: &str) -> (Option<String>, Option<String>, String) {
    let parts: Vec<&str> = name.split('.').collect();
    if parts.len() >= 3 {
        return (
            Some(parts[0].to_string()),
            Some(parts[1].to_string()),
            parts[parts.len() - 1].to_string(),
        );
    }
    if parts.len() == 2 {
        return (None, Some(parts[0].to_string()), parts[1].to_string());
    }
    (None, None, name.to_string())
}

fn is_builtin_type(name: &str) -> bool {
    matches!(
        name,
        "std_logic"
            | "std_ulogic"
            | "std_logic_vector"
            | "std_ulogic_vector"
            | "signed"
            | "unsigned"
            | "integer"
            | "natural"
            | "positive"
            | "real"
            | "time"
            | "boolean"
            | "bit"
            | "bit_vector"
            | "string"
    )
}

fn is_numeric_type(name: &str) -> bool {
    matches!(name, "integer" | "natural" | "positive" | "real" | "time")
}

fn unresolved_type_marks(input: &Input, resolver: &TypeResolver) -> Vec<Violation> {
    let mut out = Vec::new();
    let file_libs = file_library_map(input);
    let mut file_packages: HashMap<String, Vec<String>> = HashMap::new();
    for pkg in &input.packages {
        file_packages
            .entry(pkg.file.clone())
            .or_default()
            .push(pkg.name.clone());
    }

    for port in &input.ports {
        if port.r#type.trim().is_empty() {
            continue;
        }
        if is_builtin_type(&strip_type_mark(&port.r#type).to_ascii_lowercase()) {
            continue;
        }
        let port_file = if !port.file.is_empty() {
            port.file.clone()
        } else {
            match entity_file_for_name(input, &port.in_entity) {
                Some(file) => file,
                None => continue,
            }
        };
        if resolver
            .resolve_type(&port_file, "", &port.r#type)
            .is_none()
        {
            out.push(Violation {
                rule: "unresolved_type_mark".to_string(),
                severity: "error".to_string(),
                file: port_file,
                line: port.line,
                message: format!(
                    "Type '{}' for port '{}' could not be resolved via visible packages",
                    port.r#type, port.name
                ),
            });
        }
    }

    for ent in &input.entities {
        for gen in &ent.generics {
            if gen.r#type.trim().is_empty() {
                continue;
            }
            if is_builtin_type(&strip_type_mark(&gen.r#type).to_ascii_lowercase()) {
                continue;
            }
            if resolver
                .resolve_type(&ent.file, &ent.name, &gen.r#type)
                .is_none()
            {
                out.push(Violation {
                    rule: "unresolved_type_mark".to_string(),
                    severity: "error".to_string(),
                    file: ent.file.clone(),
                    line: gen.line,
                    message: format!(
                        "Type '{}' for generic '{}' could not be resolved via visible packages",
                        gen.r#type, gen.name
                    ),
                });
            }
        }
    }

    for comp in &input.components {
        for gen in &comp.generics {
            if gen.r#type.trim().is_empty() {
                continue;
            }
            if is_builtin_type(&strip_type_mark(&gen.r#type).to_ascii_lowercase()) {
                continue;
            }
            let mut resolved = resolver
                .resolve_type(&comp.file, &comp.name, &gen.r#type)
                .is_some();
            if !resolved {
                if let Some(pkgs) = file_packages.get(&comp.file) {
                    let lib = file_libs
                        .get(&comp.file)
                        .cloned()
                        .unwrap_or_else(|| "work".to_string());
                    for pkg in pkgs {
                        let type_name = strip_type_mark(&gen.r#type).to_ascii_lowercase();
                        if resolver
                            .resolve_in_package(&lib, pkg, &type_name, &comp.file, &comp.name)
                            .is_some()
                        {
                            resolved = true;
                            break;
                        }
                    }
                }
            }
            if !resolved {
                out.push(Violation {
                    rule: "unresolved_type_mark".to_string(),
                    severity: "error".to_string(),
                    file: comp.file.clone(),
                    line: gen.line,
                    message: format!(
                        "Type '{}' for generic '{}' could not be resolved via visible packages",
                        gen.r#type, gen.name
                    ),
                });
            }
        }
    }

    out
}

#[derive(Default)]
struct BindingResult {
    entity: Option<Entity>,
    arch: Option<Architecture>,
    status: BindingStatus,
}

#[derive(Default, PartialEq)]
enum BindingStatus {
    #[default]
    Resolved,
    Missing,
    Ambiguous,
}

fn component_decl_visible(
    input: &Input,
    visibility: &HashMap<String, Visibility>,
    file_packages: &HashMap<String, Vec<String>>,
    file_libs: &HashMap<String, String>,
    inst_file: &str,
    name: &str,
) -> bool {
    let vis = match visibility.get(inst_file) {
        Some(v) => v,
        None => return false,
    };
    for comp in &input.components {
        if comp.is_instance {
            continue;
        }
        if !comp.name.eq_ignore_ascii_case(name) {
            continue;
        }
        if comp.file == inst_file {
            return true;
        }
        let pkgs = match file_packages.get(&comp.file) {
            Some(p) => p,
            None => continue,
        };
        let lib = file_libs
            .get(&comp.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string());
        for pkg in pkgs {
            let key = format!("{}.{}", lib.to_ascii_lowercase(), pkg.to_ascii_lowercase());
            if vis.visible_packages.contains(&key) {
                return true;
            }
        }
    }
    false
}

fn build_binding_table(
    input: &Input,
    visibility: &HashMap<String, Visibility>,
) -> HashMap<usize, BindingResult> {
    let file_libs = file_library_map(input);
    let mut file_packages: HashMap<String, Vec<String>> = HashMap::new();
    for pkg in &input.packages {
        file_packages
            .entry(pkg.file.clone())
            .or_default()
            .push(pkg.name.clone());
    }
    let mut entity_map: HashMap<(String, String), Vec<Entity>> = HashMap::new();
    for ent in &input.entities {
        let lib = file_libs
            .get(&ent.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string());
        entity_map
            .entry((lib, ent.name.to_ascii_lowercase()))
            .or_default()
            .push(ent.clone());
    }

    let mut arch_map: HashMap<(String, String, String), Vec<Architecture>> = HashMap::new();
    for arch in &input.architectures {
        let lib = file_libs
            .get(&arch.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string())
            .to_ascii_lowercase();
        let key = (
            lib,
            arch.entity_name.to_ascii_lowercase(),
            arch.name.to_ascii_lowercase(),
        );
        arch_map.entry(key).or_default().push(arch.clone());
    }

    let mut bindings = HashMap::new();

    for (idx, inst) in input.instances.iter().enumerate() {
        if helpers::is_third_party_file(input, &inst.file) {
            continue;
        }
        let (lib_opt, name) = split_target(&inst.target);
        let mut lib = lib_opt.clone().unwrap_or_else(|| {
            file_libs
                .get(&inst.file)
                .cloned()
                .unwrap_or_else(|| "work".to_string())
        });
        if lib.eq_ignore_ascii_case("work") {
            if let Some(mapped) = file_libs.get(&inst.file) {
                lib = mapped.clone();
            }
        }

        let mut result = BindingResult::default();
        let mut candidates = entity_map
            .get(&(lib.clone(), name.to_ascii_lowercase()))
            .cloned()
            .unwrap_or_default();

        if candidates.is_empty() && lib_opt.is_none() {
            if let Some(vis) = visibility.get(&inst.file) {
                let mut visible_hits: Vec<Entity> = Vec::new();
                for lib_name in &vis.visible_libraries {
                    let mapped = if lib_name.eq_ignore_ascii_case("work") {
                        lib.clone()
                    } else {
                        lib_name.to_ascii_lowercase()
                    };
                    if let Some(list) = entity_map.get(&(mapped, name.to_ascii_lowercase())) {
                        visible_hits.extend(list.clone());
                    }
                }
                candidates = visible_hits;
            }
        }
        if candidates.is_empty() && lib_opt.is_none() {
            let is_testbench = helpers::file_in_testbench(input, &inst.file)
                || file_libs
                    .get(&inst.file)
                    .map(|lib| lib.eq_ignore_ascii_case("test"))
                    .unwrap_or(false);
            if is_testbench {
                let mut any_hits = Vec::new();
                for ((_, ent_name), list) in &entity_map {
                    if ent_name.eq_ignore_ascii_case(&name) {
                        any_hits.extend(list.clone());
                    }
                }
                candidates = any_hits;
            }
        }

        if candidates.is_empty() {
            if lib_opt.is_none()
                && component_decl_visible(
                    input,
                    visibility,
                    &file_packages,
                    &file_libs,
                    &inst.file,
                    &name,
                )
            {
                result.status = BindingStatus::Resolved;
                bindings.insert(idx, result);
                continue;
            }
            result.status = BindingStatus::Missing;
            bindings.insert(idx, result);
            continue;
        }
        if candidates.len() > 1 {
            let primary: Vec<Entity> = candidates
                .iter()
                .filter(|ent| !helpers::is_stub_file(&ent.file))
                .cloned()
                .collect();
            if primary.len() == 1 {
                candidates = primary;
            } else if primary.len() > 1 {
                candidates = primary;
            }
        }
        if candidates.len() > 1 {
            result.status = BindingStatus::Ambiguous;
            bindings.insert(idx, result);
            continue;
        }

        let ent = candidates[0].clone();
        result.entity = Some(ent.clone());

        let ent_lib = file_libs
            .get(&ent.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string())
            .to_ascii_lowercase();
        let arch_name = inst.target_arch.to_ascii_lowercase();
        if !arch_name.is_empty() {
            let key = (
                ent_lib.clone(),
                ent.name.to_ascii_lowercase(),
                arch_name,
            );
            if let Some(list) = arch_map.get(&key) {
                if list.len() == 1 {
                    result.arch = Some(list[0].clone());
                } else if !list.is_empty() {
                    result.status = BindingStatus::Ambiguous;
                } else {
                    result.status = BindingStatus::Missing;
                }
            } else {
                result.status = BindingStatus::Missing;
            }
            bindings.insert(idx, result);
            continue;
        }

        let mut arch_candidates: Vec<Architecture> = input
            .architectures
            .iter()
            .filter(|a| a.entity_name.eq_ignore_ascii_case(&ent.name))
            .filter(|a| {
                file_libs
                    .get(&a.file)
                    .map(|lib| lib.eq_ignore_ascii_case(&ent_lib))
                    .unwrap_or(false)
            })
            .cloned()
            .collect();
        if arch_candidates.len() == 1 {
            result.arch = Some(arch_candidates.remove(0));
        } else if arch_candidates.is_empty() {
            result.status = BindingStatus::Missing;
        }

        bindings.insert(idx, result);
    }

    bindings
}

fn instance_binding_violations(
    input: &Input,
    bindings: &HashMap<usize, BindingResult>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    for (idx, inst) in input.instances.iter().enumerate() {
        let result = match bindings.get(&idx) {
            Some(r) => r,
            None => continue,
        };
        match result.status {
            BindingStatus::Missing => out.push(Violation {
                rule: "unresolved_instance_binding".to_string(),
                severity: "error".to_string(),
                file: inst.file.clone(),
                line: inst.line,
                message: format!(
                    "Instance '{}' could not resolve target '{}'",
                    inst.name, inst.target
                ),
            }),
            BindingStatus::Ambiguous => out.push(Violation {
                rule: "ambiguous_instance_binding".to_string(),
                severity: "error".to_string(),
                file: inst.file.clone(),
                line: inst.line,
                message: format!(
                    "Instance '{}' target '{}' resolves to multiple entities",
                    inst.name, inst.target
                ),
            }),
            BindingStatus::Resolved => {}
        }
    }
    out
}

fn port_map_conformance(
    input: &Input,
    resolver: &TypeResolver,
    bindings: &HashMap<usize, BindingResult>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut skips = SkipTracker::default();
    for (idx, inst) in input.instances.iter().enumerate() {
        let result = match bindings.get(&idx) {
            Some(r) => r,
            None => continue,
        };
        let entity = match &result.entity {
            Some(e) => e,
            None => continue,
        };
        if helpers::is_third_party_file(input, &inst.file)
            || helpers::is_third_party_file(input, &entity.file)
            || helpers::is_stub_file(&entity.file)
        {
            continue;
        }
        let port_map = build_port_map(inst, entity);

        for extra in &port_map.extra_formals {
            out.push(Violation {
                rule: "extra_port_connection".to_string(),
                severity: "error".to_string(),
                file: inst.file.clone(),
                line: inst.line,
                message: format!(
                    "Instance '{}' connects unknown port '{}' on entity '{}'",
                    inst.name, extra, entity.name
                ),
            });
        }

        for port in &entity.ports {
            let actual_entry = port_map
                .actuals
                .get(&port.name.to_ascii_lowercase())
                .cloned();
            let actual = actual_entry
                .as_ref()
                .map(|entry| entry.actual.clone())
                .unwrap_or_default();
            if actual_is_open(&actual) {
                let dir = port.direction.to_ascii_lowercase();
                if dir == "out" || dir == "buffer" {
                    continue;
                }
                if port.default.trim().is_empty() {
                    out.push(Violation {
                        rule: "missing_port_connection".to_string(),
                        severity: "error".to_string(),
                        file: inst.file.clone(),
                        line: inst.line,
                        message: format!(
                            "Instance '{}' missing connection for port '{}' of entity '{}'",
                            inst.name, port.name, entity.name
                        ),
                    });
                }
                continue;
            }

            if let Some(msg) = port_direction_mismatch(input, inst, entity, port, &actual) {
                out.push(Violation {
                    rule: "port_direction_mismatch".to_string(),
                    severity: "error".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: msg,
                });
            }

            let dir = port.direction.to_ascii_lowercase();
            if dir != "out" && dir != "buffer" {
                let formal = actual_entry.as_ref().map(|entry| entry.formal.as_str());
                let mismatch =
                    port_type_mismatch(input, resolver, inst, entity, port, &actual, formal);
                if let Some(msg) = mismatch.message {
                    out.push(Violation {
                        rule: "port_type_mismatch".to_string(),
                        severity: "error".to_string(),
                        file: inst.file.clone(),
                        line: inst.line,
                        message: msg,
                    });
                } else if let Some(note) = mismatch.skip_note {
                    skips.record(&inst.file, inst.line, "port_type_mismatch", note);
                }
            }
        }
    }
    out.extend(skips.into_violations());
    out
}

fn generic_map_conformance(
    input: &Input,
    resolver: &TypeResolver,
    bindings: &HashMap<usize, BindingResult>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut skips = SkipTracker::default();
    for (idx, inst) in input.instances.iter().enumerate() {
        let result = match bindings.get(&idx) {
            Some(r) => r,
            None => continue,
        };
        let entity = match &result.entity {
            Some(e) => e,
            None => continue,
        };
        if helpers::is_third_party_file(input, &inst.file)
            || helpers::is_third_party_file(input, &entity.file)
            || helpers::is_stub_file(&entity.file)
        {
            continue;
        }

        let mut decls: HashMap<String, GenericDecl> = HashMap::new();
        for gen in &entity.generics {
            decls.insert(gen.name.to_ascii_lowercase(), gen.clone());
        }

        for (formal, _) in &inst.generic_map {
            if !decls.contains_key(&formal.to_ascii_lowercase()) {
                out.push(Violation {
                    rule: "extra_generic".to_string(),
                    severity: "error".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: format!(
                        "Instance '{}' maps unknown generic '{}' on entity '{}'",
                        inst.name, formal, entity.name
                    ),
                });
            }
        }

        for gen in &entity.generics {
            let actual = inst
                .generic_map
                .get(&gen.name)
                .or_else(|| {
                    inst.generic_map
                        .iter()
                        .find(|(k, _)| k.eq_ignore_ascii_case(&gen.name))
                        .map(|(_, v)| v)
                })
                .cloned()
                .unwrap_or_default();
            if actual.is_empty() {
                if gen.default.trim().is_empty() {
                    out.push(Violation {
                        rule: "missing_generic".to_string(),
                        severity: "error".to_string(),
                        file: inst.file.clone(),
                        line: inst.line,
                        message: format!(
                            "Instance '{}' missing generic '{}' for entity '{}'",
                            inst.name, gen.name, entity.name
                        ),
                    });
                }
                continue;
            }
            let mismatch = generic_type_mismatch(input, resolver, inst, gen, &actual);
            if let Some(msg) = mismatch.message {
                out.push(Violation {
                    rule: "generic_type_mismatch".to_string(),
                    severity: "error".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: msg,
                });
            } else if let Some(note) = mismatch.skip_note {
                skips.record(&inst.file, inst.line, "generic_type_mismatch", note);
            }
        }
    }
    out.extend(skips.into_violations());
    out
}

fn subprogram_resolution(
    input: &Input,
    resolver: &TypeResolver,
    visibility: &HashMap<String, Visibility>,
    instantiations: &[PackageInstantiation],
) -> Vec<Violation> {
    let mut out = Vec::new();
    let decls = build_subprogram_decls(input, instantiations);
    let file_libs = file_library_map(input);

    for proc in &input.processes {
        out.extend(resolve_calls(
            input,
            resolver,
            visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Function,
        ));
        out.extend(resolve_calls(
            input,
            resolver,
            visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Procedure,
        ));
    }

    out
}

fn configuration_checks(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for cfg in &input.configurations {
        if cfg.arch_name.trim().is_empty() {
            continue;
        }
        let found = input.architectures.iter().any(|arch| {
            arch.name.eq_ignore_ascii_case(&cfg.arch_name)
                && arch.entity_name.eq_ignore_ascii_case(&cfg.entity_name)
        });
        if !found {
            let available = available_arches_for_entity(input, &cfg.entity_name);
            let mut msg = format!(
                "Configuration '{}' references missing architecture '{}' for entity '{}'",
                cfg.name, cfg.arch_name, cfg.entity_name
            );
            if !available.is_empty() {
                msg.push_str(&format!(
                    ". Available architectures: {}",
                    available.join(", ")
                ));
            }
            out.push(Violation {
                rule: "configuration_missing_arch".to_string(),
                severity: "error".to_string(),
                file: cfg.file.clone(),
                line: cfg.line,
                message: msg,
            });
        }
    }

    for binding in &input.configuration_bindings {
        if binding.target_arch.trim().is_empty() {
            continue;
        }
        let target_entity = base_entity_name(&binding.target_entity);
        let found = input.architectures.iter().any(|arch| {
            arch.name.eq_ignore_ascii_case(&binding.target_arch)
                && arch.entity_name.eq_ignore_ascii_case(&target_entity)
        });
        if !found {
            let available = available_arches_for_entity(input, &target_entity);
            let mut msg = format!(
                "Configuration binding for '{}' references missing architecture '{}'",
                binding.component_name, binding.target_arch
            );
            if !available.is_empty() {
                msg.push_str(&format!(
                    ". Available architectures for '{}': {}",
                    target_entity,
                    available.join(", ")
                ));
            }
            out.push(Violation {
                rule: "configuration_missing_arch".to_string(),
                severity: "error".to_string(),
                file: binding.file.clone(),
                line: binding.line,
                message: msg,
            });
        }
    }

    let mut instances_by_arch_label: HashMap<(String, String), Vec<&Instance>> = HashMap::new();
    for inst in &input.instances {
        let arch = inst.in_arch.trim();
        let label = inst.name.trim();
        if arch.is_empty() || label.is_empty() {
            continue;
        }
        instances_by_arch_label
            .entry((arch.to_ascii_lowercase(), label.to_ascii_lowercase()))
            .or_default()
            .push(inst);
    }

    for binding in &input.configuration_bindings {
        if helpers::is_third_party_file(input, &binding.file) {
            continue;
        }
        let scope_arch = binding
            .scope_path
            .first()
            .map(|s| s.trim().to_string())
            .unwrap_or_default();
        if scope_arch.is_empty() {
            continue;
        }
        let instance_label = binding.instance_label.trim();
        if !instance_label.is_empty()
            && !instance_label.eq_ignore_ascii_case("all")
            && !instance_label.eq_ignore_ascii_case("others")
        {
            let key = (
                scope_arch.to_ascii_lowercase(),
                instance_label.to_ascii_lowercase(),
            );
            if !instances_by_arch_label.contains_key(&key) {
                let mut msg = format!(
                    "Configuration binding refers to instance '{}' in architecture '{}' but no such instance exists",
                    instance_label, scope_arch
                );
                if !binding.component_name.trim().is_empty() {
                    msg.push_str(&format!(
                        ". Component '{}'",
                        binding.component_name.trim()
                    ));
                }
                out.push(Violation {
                    rule: "configuration_binding_missing_instance".to_string(),
                    severity: "error".to_string(),
                    file: binding.file.clone(),
                    line: binding.line,
                    message: msg,
                });
            }
        }
        if binding.target_arch.trim().is_empty() {
            continue;
        }
        let key = (
            scope_arch.to_ascii_lowercase(),
            instance_label.to_ascii_lowercase(),
        );
        let candidates = match instances_by_arch_label.get(&key) {
            Some(list) => list,
            None => continue,
        };
        for inst in candidates {
            if !binding.component_name.trim().is_empty() {
                let inst_target = base_entity_name(inst.target.trim());
                if !inst_target.eq_ignore_ascii_case(binding.component_name.trim()) {
                    continue;
                }
            }
            if inst.target_arch.trim().is_empty() {
                continue;
            }
            if !inst
                .target_arch
                .eq_ignore_ascii_case(binding.target_arch.trim())
            {
                let mut msg = format!(
                    "Configuration binding for instance '{}' expects architecture '{}' but instantiation uses '{}'",
                    binding.instance_label, binding.target_arch, inst.target_arch
                );
                if !binding.component_name.trim().is_empty() {
                    msg.push_str(&format!(
                        ". Instance '{}' binds component '{}'",
                        binding.instance_label, binding.component_name
                    ));
                }
                if !scope_arch.is_empty() {
                    msg.push_str(&format!(". Scope: arch {}", scope_arch));
                }
                out.push(Violation {
                    rule: "configuration_binding_mismatch".to_string(),
                    severity: "error".to_string(),
                    file: binding.file.clone(),
                    line: binding.line,
                    message: msg,
                });
            }
        }
    }

    out
}

fn duplicate_architecture_in_library(
    input: &Input,
    file_libs: &HashMap<String, String>,
) -> Vec<Violation> {
    let mut by_key: HashMap<(String, String, String), Vec<&Architecture>> = HashMap::new();
    for arch in &input.architectures {
        if arch.name.trim().is_empty() || arch.entity_name.trim().is_empty() {
            continue;
        }
        if helpers::is_third_party_file(input, &arch.file) {
            continue;
        }
        let lib = file_libs
            .get(&arch.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string())
            .to_ascii_lowercase();
        let key = (
            lib,
            arch.entity_name.to_ascii_lowercase(),
            arch.name.to_ascii_lowercase(),
        );
        by_key.entry(key).or_default().push(arch);
    }

    let mut out = Vec::new();
    for ((lib, _, _), mut arches) in by_key {
        if arches.len() <= 1 {
            continue;
        }
        arches.sort_by(|a, b| a.file.cmp(&b.file).then_with(|| a.line.cmp(&b.line)));
        let first = arches[0];
        for dup in arches.iter().skip(1) {
            out.push(Violation {
                rule: "duplicate_architecture_in_library".to_string(),
                severity: "error".to_string(),
                file: dup.file.clone(),
                line: dup.line,
                message: format!(
                    "Duplicate architecture '{}' for entity '{}' in library '{}' (also defined at {}:{})",
                    first.name, first.entity_name, lib, first.file, first.line
                ),
            });
        }
    }
    out
}

fn package_body_without_declaration(
    input: &Input,
    file_libs: &HashMap<String, String>,
) -> Vec<Violation> {
    let mut declared = HashSet::new();
    for pkg in &input.packages {
        if pkg.name.trim().is_empty() {
            continue;
        }
        let lib = file_libs
            .get(&pkg.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string());
        let key = format!(
            "{}.{}",
            lib.to_ascii_lowercase(),
            pkg.name.to_ascii_lowercase()
        );
        declared.insert(key);
    }

    let mut out = Vec::new();
    for body in &input.package_bodies {
        if body.name.trim().is_empty() {
            continue;
        }
        let lib = file_libs
            .get(&body.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string());
        let key = format!(
            "{}.{}",
            lib.to_ascii_lowercase(),
            body.name.to_ascii_lowercase()
        );
        if declared.contains(&key) {
            continue;
        }
        out.push(Violation {
            rule: "package_body_without_declaration".to_string(),
            severity: "error".to_string(),
            file: body.file.clone(),
            line: body.line,
            message: format!(
                "Package body '{}' has no matching package declaration in library '{}'",
                body.name, lib
            ),
        });
    }

    out
}

fn cross_file_assignment_checks(
    input: &Input,
    resolver: &TypeResolver,
    bindings: &HashMap<usize, BindingResult>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    for (idx, inst) in input.instances.iter().enumerate() {
        let result = match bindings.get(&idx) {
            Some(r) => r,
            None => continue,
        };
        let entity = match &result.entity {
            Some(e) => e,
            None => continue,
        };
        let port_map = build_port_map(inst, entity);
        for port in &entity.ports {
            let dir = port.direction.to_ascii_lowercase();
            if dir != "out" && dir != "buffer" {
                continue;
            }
            let actual_entry = port_map
                .actuals
                .get(&port.name.to_ascii_lowercase())
                .cloned();
            let actual = actual_entry
                .as_ref()
                .map(|entry| entry.actual.clone())
                .unwrap_or_default();
            if actual_is_open(&actual) {
                continue;
            }
            let formal = actual_entry.as_ref().map(|entry| entry.formal.as_str());
            let mismatch = port_type_mismatch(input, resolver, inst, entity, port, &actual, formal);
            if let Some(msg) = mismatch.message {
                out.push(Violation {
                    rule: "cross_file_assignment_type_mismatch".to_string(),
                    severity: "error".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: msg,
                });
            }
        }
    }
    out
}

#[derive(Clone)]
struct PortActual {
    actual: String,
    formal: String,
}

struct PortMapInfo {
    actuals: HashMap<String, PortActual>,
    extra_formals: Vec<String>,
}

fn build_port_map(inst: &Instance, entity: &Entity) -> PortMapInfo {
    let mut actuals: HashMap<String, PortActual> = HashMap::new();
    let mut extra = Vec::new();
    let mut extra_set = HashSet::new();
    let ports: Vec<String> = entity
        .ports
        .iter()
        .map(|p| p.name.to_ascii_lowercase())
        .collect();

    for assoc in &inst.associations {
        if assoc.kind != "port" {
            continue;
        }
        if assoc.is_positional {
            if assoc.position_index < ports.len() {
                let formal = ports[assoc.position_index].clone();
                actuals.insert(
                    formal.clone(),
                    PortActual {
                        actual: assoc.actual.clone(),
                        formal,
                    },
                );
            } else {
                let label = format!("positional@{}", assoc.position_index);
                if extra_set.insert(label.clone()) {
                    extra.push(label);
                }
            }
            continue;
        }
        let formal_raw = assoc.formal.clone();
        let formal_key = base_name(&formal_raw).to_ascii_lowercase();
        if ports.iter().any(|p| p == &formal_key) {
            actuals.insert(
                formal_key,
                PortActual {
                    actual: assoc.actual.clone(),
                    formal: formal_raw,
                },
            );
        } else {
            let label = assoc.formal.clone();
            if extra_set.insert(label.clone()) {
                extra.push(label);
            }
        }
    }

    for (formal, actual) in &inst.port_map {
        let key = base_name(formal).to_ascii_lowercase();
        if actuals.contains_key(&key) {
            continue;
        }
        if ports.iter().any(|p| p == &key) {
            actuals.insert(
                key.clone(),
                PortActual {
                    actual: actual.clone(),
                    formal: formal.clone(),
                },
            );
        } else {
            if extra_set.insert(formal.clone()) {
                extra.push(formal.clone());
            }
        }
    }

    PortMapInfo {
        actuals,
        extra_formals: extra,
    }
}

fn port_direction_mismatch(
    input: &Input,
    inst: &Instance,
    entity: &Entity,
    port: &Port,
    actual: &str,
) -> Option<String> {
    let base = base_name(actual);
    if base.is_empty() {
        return None;
    }
    let parent_ports = parent_port_map(input, inst);
    let actual_dir = parent_ports.get(&base.to_ascii_lowercase())?;

    if port.direction.eq_ignore_ascii_case("out")
        || port.direction.eq_ignore_ascii_case("inout")
        || port.direction.eq_ignore_ascii_case("buffer")
    {
        if actual_dir.eq_ignore_ascii_case("in") {
            return Some(format!(
                "Instance '{}' drives parent input port '{}' via output '{}' of entity '{}'",
                inst.name, base, port.name, entity.name
            ));
        }
    }

    if port.direction.eq_ignore_ascii_case("in") {
        if actual_dir.eq_ignore_ascii_case("out") || actual_dir.eq_ignore_ascii_case("buffer") {
            return Some(format!(
                "Instance '{}' reads parent output port '{}' via input '{}' of entity '{}'",
                inst.name, base, port.name, entity.name
            ));
        }
    }

    None
}

struct MismatchResult {
    message: Option<String>,
    skip_note: Option<String>,
}

fn port_type_mismatch(
    input: &Input,
    resolver: &TypeResolver,
    inst: &Instance,
    entity: &Entity,
    port: &Port,
    actual: &str,
    formal: Option<&str>,
) -> MismatchResult {
    let actual_type = match resolve_actual_type(input, resolver, inst, actual) {
        Some(t) => t,
        None => {
            return MismatchResult {
                message: None,
                skip_note: Some(format!(
                    "unable to resolve type for actual '{}' in instance '{}'",
                    actual, inst.name
                )),
            }
        }
    };
    let port_type = match resolver.resolve_type(&entity.file, "", &port.r#type) {
        Some(t) => t,
        None => {
            return MismatchResult {
                message: None,
                skip_note: Some(format!(
                    "unable to resolve type for port '{}.{}'",
                    entity.name, port.name
                )),
            }
        }
    };
    let expected_type = match formal_access_kind(formal) {
        FormalAccessKind::Element => resolve_indexed_type(resolver, &entity.file, "", &port_type)
            .unwrap_or_else(|| port_type.clone()),
        FormalAccessKind::Slice | FormalAccessKind::None => port_type.clone(),
    };
    if types_compatible(resolver, &actual_type, &expected_type) {
        return MismatchResult {
            message: None,
            skip_note: None,
        };
    }
    let mut msg = format!(
        "Type mismatch: actual '{}' has type {} but port '{}.{}' expects {}",
        actual,
        describe_type(&actual_type),
        entity.name,
        port.name,
        describe_type(&expected_type)
    );
    if let Some(hint) = type_compat_hint(&actual_type, &expected_type) {
        msg.push_str(&format!(". Hint: {}", hint));
    }
    MismatchResult {
        message: Some(msg),
        skip_note: None,
    }
}

enum FormalAccessKind {
    None,
    Element,
    Slice,
}

fn formal_access_kind(formal: Option<&str>) -> FormalAccessKind {
    let raw = match formal {
        Some(val) => val,
        None => return FormalAccessKind::None,
    };
    let start = match raw.find('(') {
        Some(idx) => idx,
        None => return FormalAccessKind::None,
    };
    let end = match raw.rfind(')') {
        Some(idx) if idx > start => idx,
        _ => return FormalAccessKind::None,
    };
    let content = &raw[start + 1..end];
    if is_slice_content(content) {
        FormalAccessKind::Slice
    } else {
        FormalAccessKind::Element
    }
}

fn generic_type_mismatch(
    input: &Input,
    resolver: &TypeResolver,
    inst: &Instance,
    gen: &GenericDecl,
    actual: &str,
) -> MismatchResult {
    let actual_type = match infer_expr_type(input, resolver, inst, actual) {
        Some(t) => t,
        None => {
            return MismatchResult {
                message: None,
                skip_note: Some(format!(
                    "unable to resolve type for generic actual '{}' in instance '{}'",
                    actual, inst.name
                )),
            }
        }
    };
    let entity_file = match entity_file_for_name(input, &gen.in_entity) {
        Some(f) => f,
        None => {
            return MismatchResult {
                message: None,
                skip_note: Some(format!(
                    "unable to resolve entity file for generic '{}'",
                    gen.name
                )),
            }
        }
    };
    let gen_type = match resolver.resolve_type(&entity_file, &gen.in_entity, &gen.r#type) {
        Some(t) => t,
        None => {
            return MismatchResult {
                message: None,
                skip_note: Some(format!(
                    "unable to resolve type for generic '{}.{}'",
                    gen.in_entity, gen.name
                )),
            }
        }
    };
    let actual_token = normalize_literal_token(actual);
    if (actual_token == "true" || actual_token == "false")
        && enum_literal_matches_type(resolver, &inst.file, &inst.in_arch, &gen_type, &actual_token)
    {
        return MismatchResult {
            message: None,
            skip_note: None,
        };
    }
    if types_compatible(resolver, &actual_type, &gen_type) {
        return MismatchResult {
            message: None,
            skip_note: None,
        };
    }
    let mut msg = format!(
        "Generic '{}' actual '{}' type mismatch ({} vs {})",
        gen.name,
        actual,
        describe_type(&actual_type),
        describe_type(&gen_type)
    );
    if let Some(hint) = type_compat_hint(&actual_type, &gen_type) {
        msg.push_str(&format!(". Hint: {}", hint));
    }
    MismatchResult {
        message: Some(msg),
        skip_note: None,
    }
}

fn types_compatible(resolver: &TypeResolver, a: &ResolvedType, b: &ResolvedType) -> bool {
    if std_logic_compatible(a, b) {
        return true;
    }
    if array_std_logic_vector_compatible(a, b) {
        return true;
    }
    if numeric_array_compatible(resolver, a, b) {
        return true;
    }
    if numeric_compatible(resolver, a, b) {
        return true;
    }
    if !a.canonical.eq_ignore_ascii_case(&b.canonical) {
        return false;
    }
    if let (Some(ac), Some(bc)) = (&a.constraint, &b.constraint) {
        return ac.eq_ignore_ascii_case(bc);
    }
    true
}

fn numeric_compatible(resolver: &TypeResolver, a: &ResolvedType, b: &ResolvedType) -> bool {
    is_numeric_like(resolver, a) && is_numeric_like(resolver, b)
}

fn numeric_array_compatible(resolver: &TypeResolver, a: &ResolvedType, b: &ResolvedType) -> bool {
    if !a.kind.eq_ignore_ascii_case("array") || !b.kind.eq_ignore_ascii_case("array") {
        return false;
    }
    let a_elem = a
        .element_type
        .as_ref()
        .map(|s| s.to_ascii_lowercase())
        .unwrap_or_default();
    let b_elem = b
        .element_type
        .as_ref()
        .map(|s| s.to_ascii_lowercase())
        .unwrap_or_default();
    !a_elem.is_empty()
        && !b_elem.is_empty()
        && is_numeric_like_name(resolver, &a_elem)
        && is_numeric_like_name(resolver, &b_elem)
}

fn is_numeric_like(resolver: &TypeResolver, ty: &ResolvedType) -> bool {
    if ty.kind.eq_ignore_ascii_case("numeric") || ty.kind.eq_ignore_ascii_case("physical") {
        return true;
    }
    let canon = ty.canonical.to_ascii_lowercase();
    is_numeric_type(&canon) || resolver.is_physical_type_name(&canon)
}

fn is_numeric_like_name(resolver: &TypeResolver, name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    is_numeric_type(&lower) || resolver.is_physical_type_name(&lower)
}

fn normalize_constraint(constraint: &str) -> String {
    constraint
        .split_whitespace()
        .collect::<Vec<&str>>()
        .join(" ")
        .to_ascii_lowercase()
}

fn describe_type(ty: &ResolvedType) -> String {
    match &ty.constraint {
        Some(c) if !c.is_empty() => format!("{} [{}]", ty.canonical, c),
        _ => ty.canonical.clone(),
    }
}

fn type_compat_hint(actual: &ResolvedType, expected: &ResolvedType) -> Option<String> {
    let a = actual.canonical.to_ascii_lowercase();
    let b = expected.canonical.to_ascii_lowercase();
    if a.contains("signed") && b.contains("unsigned") {
        return Some("cast with unsigned(...) or std_logic_vector(...)".to_string());
    }
    if a.contains("unsigned") && b.contains("signed") {
        return Some("cast with signed(...) or std_logic_vector(...)".to_string());
    }
    if (a.contains("std_logic_vector") && b == "std_logic")
        || (b.contains("std_logic_vector") && a == "std_logic")
    {
        return Some("adjust scalar/vector width or index the vector".to_string());
    }
    if let (Some(ac), Some(bc)) = (&actual.constraint, &expected.constraint) {
        if !ac.eq_ignore_ascii_case(bc) {
            return Some(format!("constraints differ ({} vs {})", ac, bc));
        }
    }
    None
}

fn std_logic_compatible(a: &ResolvedType, b: &ResolvedType) -> bool {
    let a = a.canonical.to_ascii_lowercase();
    let b = b.canonical.to_ascii_lowercase();
    matches!(
        (a.as_str(), b.as_str()),
        ("std_logic", "std_ulogic")
            | ("std_ulogic", "std_logic")
            | ("std_logic_vector", "std_ulogic_vector")
            | ("std_ulogic_vector", "std_logic_vector")
    )
}

fn array_std_logic_vector_compatible(a: &ResolvedType, b: &ResolvedType) -> bool {
    let a_vec = matches!(
        a.canonical.to_ascii_lowercase().as_str(),
        "std_logic_vector" | "std_ulogic_vector"
    );
    let b_vec = matches!(
        b.canonical.to_ascii_lowercase().as_str(),
        "std_logic_vector" | "std_ulogic_vector"
    );
    let a_array = a.kind.eq_ignore_ascii_case("array")
        && a.element_type
            .as_ref()
            .map(|t| matches!(t.to_ascii_lowercase().as_str(), "std_logic" | "std_ulogic"))
            .unwrap_or(false);
    let b_array = b.kind.eq_ignore_ascii_case("array")
        && b.element_type
            .as_ref()
            .map(|t| matches!(t.to_ascii_lowercase().as_str(), "std_logic" | "std_ulogic"))
            .unwrap_or(false);
    (a_vec && b_array) || (b_vec && a_array)
}

fn logic_vector_compatible(actual: &str, param: &str) -> bool {
    let a = actual.to_ascii_lowercase();
    let p = param.to_ascii_lowercase();
    (a == "std_logic_vector" && p == "std_ulogic_vector")
        || (a == "std_ulogic_vector" && p == "std_logic_vector")
}

fn normalize_literal_token(raw: &str) -> String {
    raw.trim()
        .trim_end_matches(|c: char| c == ',' || c == ')' || c == ';')
        .to_ascii_lowercase()
}

fn enum_literal_matches_type(
    resolver: &TypeResolver,
    file: &str,
    arch: &str,
    enum_type: &ResolvedType,
    literal: &str,
) -> bool {
    if !enum_type.kind.eq_ignore_ascii_case("enum") {
        return false;
    }
    let literal_lower = literal.to_ascii_lowercase();
    let canon = enum_type.canonical.to_ascii_lowercase();
    let parts: Vec<&str> = canon.split('.').collect();
    if parts.len() >= 3 {
        let lib = parts[0];
        let pkg = parts[1];
        let name = parts[parts.len() - 1];
        let key = format!("{}.{}", lib, pkg);
        if let Some(map) = resolver.pkg_enum_literals.get(&key) {
            if let Some(ty) = map.get(&literal_lower) {
                if ty.eq_ignore_ascii_case(name) {
                    return true;
                }
            }
        }
    }
    if !arch.is_empty() {
        let key = (file.to_string(), arch.to_ascii_lowercase());
        if let Some(map) = resolver.arch_enum_literals.get(&key) {
            if let Some(ty) = map.get(&literal_lower) {
                if ty.eq_ignore_ascii_case(&canon) {
                    return true;
                }
            }
        }
    }
    if let Some(vis) = resolver.visibility.get(file) {
        for pkg_key in &vis.visible_packages {
            if let Some(map) = resolver.pkg_enum_literals.get(pkg_key) {
                if let Some(ty) = map.get(&literal_lower) {
                    if ty.eq_ignore_ascii_case(&canon) {
                        return true;
                    }
                }
            }
        }
    }
    false
}

#[derive(Default)]
struct SkipTracker {
    seen: HashSet<String>,
    notes: Vec<Violation>,
}

impl SkipTracker {
    fn record(&mut self, file: &str, line: usize, rule: &str, note: String) {
        let key = format!("{}|{}|{}", rule, file, note);
        if self.seen.insert(key) {
            self.notes.push(Violation {
                rule: "rule_skipped".to_string(),
                severity: "info".to_string(),
                file: file.to_string(),
                line,
                message: format!("{} skipped: {}", rule, note),
            });
        }
    }

    fn into_violations(self) -> Vec<Violation> {
        self.notes
    }
}

fn call_severity(input: &Input, proc: &Process, default: &str) -> String {
    if helpers::process_in_testbench(input, proc) {
        return "warning".to_string();
    }
    default.to_string()
}

fn format_visible_packages(vis: &Visibility, limit: usize) -> String {
    let mut list: Vec<String> = vis.visible_packages.iter().cloned().collect();
    list.sort();
    format_list(&list, limit)
}

fn find_all_named_subprograms(
    decls: &[SubprogramDecl],
    kind: CallKind,
    name: &str,
) -> Vec<SubprogramDecl> {
    decls
        .iter()
        .filter(|d| d.kind == kind.as_decl_kind() && d.name.eq_ignore_ascii_case(name))
        .cloned()
        .collect()
}

fn summarize_subprograms(decls: &[SubprogramDecl], limit: usize) -> String {
    let mut items: Vec<String> = decls.iter().map(format_subprogram_signature).collect();
    items.sort();
    items.dedup();
    format_list(&items, limit)
}

fn dedup_subprogram_matches(matches: &[SubprogramDecl]) -> Vec<SubprogramDecl> {
    let mut seen: HashSet<String> = HashSet::new();
    let mut out = Vec::new();
    for m in matches {
        let sig = format_subprogram_signature(m).to_ascii_lowercase();
        if seen.insert(sig) {
            out.push(m.clone());
        }
    }
    out
}

fn format_subprogram_signature(decl: &SubprogramDecl) -> String {
    let mut prefix = String::new();
    if !decl.library.is_empty() {
        prefix.push_str(&decl.library);
    }
    if !decl.in_package.is_empty() {
        if !prefix.is_empty() {
            prefix.push('.');
        }
        prefix.push_str(&decl.in_package);
    }
    let mut name = decl.name.clone();
    if !prefix.is_empty() {
        name = format!("{}.{}", prefix, name);
    }
    let params: Vec<String> = decl.params.iter().map(|p| p.canonical.clone()).collect();
    format!("{}({})", name, params.join(", "))
}

fn format_list(items: &[String], limit: usize) -> String {
    if items.is_empty() {
        return String::new();
    }
    if items.len() <= limit {
        return items.join(", ");
    }
    format!(
        "{}, ... (+{})",
        items[..limit].join(", "),
        items.len() - limit
    )
}

fn available_arches_for_entity(input: &Input, entity: &str) -> Vec<String> {
    let mut arches: Vec<String> = input
        .architectures
        .iter()
        .filter(|arch| arch.entity_name.eq_ignore_ascii_case(entity))
        .map(|arch| arch.name.clone())
        .collect();
    arches.sort();
    arches.dedup();
    arches
}

fn resolve_actual_type(
    input: &Input,
    resolver: &TypeResolver,
    inst: &Instance,
    actual: &str,
) -> Option<ResolvedType> {
    infer_expr_type(input, resolver, inst, actual)
}

fn infer_expr_type(
    input: &Input,
    resolver: &TypeResolver,
    inst: &Instance,
    expr: &str,
) -> Option<ResolvedType> {
    let raw = strip_association_actual(expr);
    if raw.is_empty() {
        return None;
    }
    let lower = raw.to_ascii_lowercase();
    if lower == "true" || lower == "false" {
        return Some(ResolvedType::builtin("boolean".to_string()));
    }
    if lower.starts_with("'") && lower.ends_with("'") && lower.len() == 3 {
        return Some(ResolvedType::builtin("std_logic".to_string()));
    }
    if let Some(attr) = infer_attribute_type(raw) {
        return Some(attr);
    }
    if looks_like_boolean_expr(raw) {
        return Some(ResolvedType::builtin("boolean".to_string()));
    }
    if lower.chars().all(|c| c.is_ascii_digit()) {
        return Some(ResolvedType::builtin("integer".to_string()));
    }
    if looks_like_numeric_expr(raw) {
        return Some(ResolvedType::builtin("integer".to_string()));
    }
    if lower.starts_with("x\"") || lower.starts_with("b\"") || lower.starts_with("o\"") {
        return Some(ResolvedType::builtin("std_logic_vector".to_string()));
    }
    if is_bit_string_literal(raw) {
        return Some(ResolvedType::builtin("std_logic_vector".to_string()));
    }
    if lower.starts_with('\"') && lower.ends_with('\"') && lower.len() >= 2 {
        return Some(ResolvedType::builtin("string".to_string()));
    }
    if let Some(conv) = infer_conversion_call_type(raw) {
        return Some(conv);
    }
    if let Some(qualified) = infer_qualified_expr_type(input, resolver, inst, raw) {
        return Some(qualified);
    }
    if let Some(call_ty) = infer_call_return_type(input, resolver, inst, raw) {
        return Some(call_ty);
    }
    if let Some(agg) = infer_aggregate_type(input, resolver, inst, raw) {
        return Some(agg);
    }
    if looks_like_arithmetic_expr(raw) {
        if let Some(ty) = infer_numeric_expr_type(input, resolver, inst, raw) {
            return Some(ty);
        }
        return None;
    }
    if let Some(resolved) =
        resolve_named_reference_type(input, resolver, &inst.file, &inst.in_arch, raw)
    {
        return Some(resolved);
    }
    if let Some((base, _)) = split_base_identifier(raw) {
        if let Some(resolved) =
            resolve_named_reference_type(input, resolver, &inst.file, &inst.in_arch, &base)
        {
            return Some(resolved);
        }
    }
    None
}

fn strip_association_actual(expr: &str) -> &str {
    let trimmed = expr.trim();
    let bytes = trimmed.as_bytes();
    let mut depth = 0usize;
    let mut in_string = false;
    let mut i = 0usize;
    while i + 1 < bytes.len() {
        match bytes[i] {
            b'"' => {
                in_string = !in_string;
            }
            b'(' if !in_string => depth += 1,
            b')' if !in_string && depth > 0 => depth -= 1,
            b'=' if !in_string && depth == 0 && bytes[i + 1] == b'>' => {
                return trimmed[i + 2..].trim();
            }
            _ => {}
        }
        i += 1;
    }
    trimmed
}

fn looks_like_numeric_expr(raw: &str) -> bool {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return false;
    }
    let mut saw_digit = false;
    for ch in trimmed.chars() {
        if ch.is_ascii_digit() {
            saw_digit = true;
            continue;
        }
        if ch.is_ascii_whitespace()
            || matches!(ch, '+' | '-' | '*' | '/' | '(' | ')' | '_')
        {
            continue;
        }
        return false;
    }
    saw_digit
}

fn infer_call_return_type(
    input: &Input,
    resolver: &TypeResolver,
    inst: &Instance,
    raw: &str,
) -> Option<ResolvedType> {
    let call_name = parse_call_candidate(raw)?;
    if call_name.contains('\'') {
        return None;
    }
    if resolve_named_reference_type(input, resolver, &inst.file, &inst.in_arch, call_name).is_some()
    {
        return None;
    }
    let call = parse_call_name(call_name);
    let candidates = select_subprogram_candidates(
        input,
        &resolver.visibility,
        &resolver.subprogram_decls,
        &inst.file,
        &inst.in_arch,
        &call,
        CallKind::Function,
    );
    if candidates.is_empty() {
        return None;
    }
    let mut resolved: Vec<ResolvedType> = Vec::new();
    for cand in candidates {
        if cand.return_type.trim().is_empty() {
            continue;
        }
        if let Some(r) = resolver.resolve_type(&cand.file, &cand.in_arch, &cand.return_type) {
            resolved.push(r);
        }
    }
    unique_resolved_type(&resolved)
}

fn infer_numeric_expr_type(
    input: &Input,
    resolver: &TypeResolver,
    inst: &Instance,
    raw: &str,
) -> Option<ResolvedType> {
    if !looks_like_arithmetic_expr(raw) {
        return None;
    }
    let mut has_ident = false;
    for token in extract_identifier_tokens(raw) {
        has_ident = true;
        if token.is_call {
            if let Some(ty) = infer_call_return_type(input, resolver, inst, token.name.as_str()) {
                if !is_numeric_type(&ty.canonical.to_ascii_lowercase()) {
                    return None;
                }
                continue;
            }
            return None;
        }
        if let Some(ty) =
            resolve_named_reference_type(input, resolver, &inst.file, &inst.in_arch, &token.name)
        {
            if !is_numeric_type(&ty.canonical.to_ascii_lowercase()) {
                return None;
            }
            continue;
        }
        return None;
    }
    if !has_ident {
        return None;
    }
    Some(ResolvedType::builtin("integer".to_string()))
}

struct IdentToken {
    name: String,
    is_call: bool,
}

fn extract_identifier_tokens(raw: &str) -> Vec<IdentToken> {
    let bytes = raw.as_bytes();
    let mut out = Vec::new();
    let mut idx = 0;
    let mut in_string = false;
    while idx < bytes.len() {
        let ch = bytes[idx];
        if ch == b'\"' {
            in_string = !in_string;
            idx += 1;
            continue;
        }
        if in_string {
            idx += 1;
            continue;
        }
        if !is_ident_start(ch) {
            idx += 1;
            continue;
        }
        let start = idx;
        idx += 1;
        while idx < bytes.len() && is_ident_part(bytes[idx]) {
            idx += 1;
        }
        let mut name = raw[start..idx].to_string();
        while idx < bytes.len() && bytes[idx].is_ascii_whitespace() {
            idx += 1;
        }
        while idx < bytes.len() && bytes[idx] == b'.' {
            idx += 1;
            while idx < bytes.len() && bytes[idx].is_ascii_whitespace() {
                idx += 1;
            }
            let seg_start = idx;
            if idx >= bytes.len() || !is_ident_start(bytes[idx]) {
                break;
            }
            idx += 1;
            while idx < bytes.len() && is_ident_part(bytes[idx]) {
                idx += 1;
            }
            name.push('.');
            name.push_str(&raw[seg_start..idx]);
            while idx < bytes.len() && bytes[idx].is_ascii_whitespace() {
                idx += 1;
            }
        }
        if is_keyword_token(&name) {
            continue;
        }
        let mut look = idx;
        while look < bytes.len() && bytes[look].is_ascii_whitespace() {
            look += 1;
        }
        let is_call = look < bytes.len() && bytes[look] == b'(';
        out.push(IdentToken { name, is_call });
    }
    out
}

fn is_keyword_token(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "and"
            | "or"
            | "xor"
            | "xnor"
            | "nand"
            | "nor"
            | "not"
            | "mod"
            | "rem"
            | "others"
            | "downto"
            | "to"
            | "range"
    )
}

fn infer_conversion_call_type(raw: &str) -> Option<ResolvedType> {
    let trimmed = raw.trim();
    let open = trimmed.find('(')?;
    let close = trimmed.rfind(')')?;
    if close <= open {
        return None;
    }
    let head = trimmed[..open].trim();
    if head.is_empty() {
        return None;
    }
    let name = head.split('.').last().unwrap_or(head).trim().to_ascii_lowercase();
    let ty = match name.as_str() {
        "std_logic" => "std_logic",
        "std_ulogic" => "std_ulogic",
        "std_logic_vector" => "std_logic_vector",
        "std_ulogic_vector" => "std_ulogic_vector",
        "unsigned" => "unsigned",
        "signed" => "signed",
        "to_unsigned" => "unsigned",
        "to_signed" => "signed",
        "to_integer" | "integer" => "integer",
        "natural" => "natural",
        "positive" => "positive",
        "real" => "real",
        "bit" => "bit",
        "bit_vector" => "bit_vector",
        "to_bit" => "bit",
        "to_bitvector" => "bit_vector",
        "to_stdlogic" => "std_logic",
        "to_stdulogic" => "std_ulogic",
        "to_stdlogicvector" => "std_logic_vector",
        "to_stdulogicvector" => "std_ulogic_vector",
        "boolean" => "boolean",
        "string" => "string",
        _ => return None,
    };
    Some(ResolvedType::builtin(ty.to_string()))
}

fn infer_qualified_expr_type(
    _input: &Input,
    resolver: &TypeResolver,
    inst: &Instance,
    raw: &str,
) -> Option<ResolvedType> {
    let trimmed = raw.trim();
    if trimmed.is_empty() || !trimmed.contains('\'') {
        return None;
    }
    let bytes = trimmed.as_bytes();
    for idx in 0..bytes.len() {
        if bytes[idx] != b'\'' {
            continue;
        }
        let mut look = idx + 1;
        while look < bytes.len() && bytes[look].is_ascii_whitespace() {
            look += 1;
        }
        if look >= bytes.len() || bytes[look] != b'(' {
            continue;
        }
        let type_name = trimmed[..idx].trim();
        if type_name.is_empty() {
            return None;
        }
        return resolver.resolve_type(&inst.file, &inst.in_arch, type_name).or_else(|| {
            let base = strip_type_mark(type_name);
            if base == type_name {
                None
            } else {
                resolver.resolve_type(&inst.file, &inst.in_arch, &base)
            }
        });
    }
    None
}

fn parse_call_candidate(raw: &str) -> Option<&str> {
    let trimmed = raw.trim();
    let open = trimmed.find('(')?;
    let close = trimmed.rfind(')')?;
    if close <= open {
        return None;
    }
    if !trimmed[close + 1..].trim().is_empty() {
        return None;
    }
    let head = trimmed[..open].trim();
    if head.is_empty() {
        return None;
    }
    Some(head)
}

fn unique_resolved_type(types: &[ResolvedType]) -> Option<ResolvedType> {
    if types.is_empty() {
        return None;
    }
    let mut unique = HashSet::new();
    let mut first = None;
    for ty in types {
        if first.is_none() {
            first = Some(ty.clone());
        }
        unique.insert(ty.canonical.to_ascii_lowercase());
    }
    if unique.len() == 1 {
        return first;
    }
    None
}

fn infer_attribute_type(expr: &str) -> Option<ResolvedType> {
    let trimmed = expr.trim();
    let lower = trimmed.to_ascii_lowercase();
    let idx = lower.rfind('\'')?;
    let mut attr = lower[idx + 1..].trim().to_string();
    if attr.starts_with('(') {
        return None;
    }
    if let Some(pos) = attr.find('(') {
        attr.truncate(pos);
    }
    let attr = attr.trim();
    match attr {
        "length" | "left" | "right" | "high" | "low" => {
            Some(ResolvedType::builtin("integer".to_string()))
        }
        _ => None,
    }
}

fn infer_aggregate_type(
    input: &Input,
    resolver: &TypeResolver,
    inst: &Instance,
    raw: &str,
) -> Option<ResolvedType> {
    let trimmed = raw.trim();
    if !trimmed.starts_with('(') || !trimmed.contains("=>") {
        return None;
    }
    let parts: Vec<&str> = trimmed.split("=>").collect();
    if parts.len() < 2 {
        return None;
    }
    let mut rhs = parts[parts.len() - 1].trim();
    rhs = rhs.trim_end_matches(')').trim();
    if rhs.is_empty() {
        return None;
    }
    let elem = infer_expr_type(input, resolver, inst, rhs)?;
    Some(ResolvedType {
        canonical: format!("array_of_{}", elem.canonical),
        kind: "array".to_string(),
        constraint: None,
        element_type: Some(elem.canonical),
        fields: Vec::new(),
    })
}

fn build_subprogram_decls(
    input: &Input,
    instantiations: &[PackageInstantiation],
) -> Vec<SubprogramDecl> {
    let file_libs = file_library_map(input);
    let mut out = Vec::new();
    for func in &input.functions {
        let lib = file_libs
            .get(&func.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string());
        out.push(SubprogramDecl::from_function(func, &lib));
    }
    for proc in &input.procedures {
        let lib = file_libs
            .get(&proc.file)
            .cloned()
            .unwrap_or_else(|| "work".to_string());
        out.push(SubprogramDecl::from_procedure(proc, &lib));
    }
    if !instantiations.is_empty() {
        let mut aliases = Vec::new();
        for inst in instantiations {
            let inst_lib = inst.inst_lib.clone();
            let inst_pkg = inst.inst_pkg.clone();
            let target_lib = inst.target_lib.clone();
            let target_pkg = inst.target_pkg.clone();
            for decl in &out {
                if !decl.in_package.eq_ignore_ascii_case(&target_pkg) {
                    continue;
                }
                if !decl.library.eq_ignore_ascii_case(&target_lib) {
                    continue;
                }
                let mut alias = decl.clone();
                alias.in_package = inst_pkg.clone();
                alias.library = inst_lib.clone();
                aliases.push(alias);
            }
        }
        out.extend(aliases);
    }
    out
}

fn resolve_calls(
    input: &Input,
    resolver: &TypeResolver,
    visibility: &HashMap<String, Visibility>,
    file_libs: &HashMap<String, String>,
    proc: &Process,
    decls: &[SubprogramDecl],
    kind: CallKind,
) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut skips = SkipTracker::default();
    let calls: Vec<(String, Vec<String>, usize)> = match kind {
        CallKind::Function => proc
            .function_calls
            .iter()
            .map(|call| (call.name.clone(), call.args.clone(), call.line))
            .collect(),
        CallKind::Procedure => proc
            .procedure_calls
            .iter()
            .map(|call| {
                let name = if !call.full_name.is_empty() {
                    call.full_name.clone()
                } else {
                    call.name.clone()
                };
                (name, call.args.clone(), call.line)
            })
            .collect(),
    };
    let mut call_counts: HashMap<(String, Vec<String>), Vec<usize>> = HashMap::new();
    for (name, args, line) in &calls {
        call_counts
            .entry((name.clone(), args.clone()))
            .or_default()
            .push(*line);
    }
    let filtered_calls: Vec<(String, Vec<String>, usize)> = calls
        .into_iter()
        .filter(|(name, args, line)| {
            if *line == proc.line {
                if let Some(lines) = call_counts.get(&(name.clone(), args.clone())) {
                    return lines.len() == 1;
                }
            }
            true
        })
        .collect();

    for (name, args, line) in filtered_calls {
        let call = parse_call_name(&name);
        if is_object_call_prefix(input, proc, &call) {
            continue;
        }
        if is_standard_qualified_call(&call) {
            continue;
        }
        if call.pkg.is_none() && call.lib.is_none() {
            let lower = call.name.to_ascii_lowercase();
            if is_builtin_type(&lower) {
                continue;
            }
        }
        let candidates = select_subprogram_candidates(
            input,
            visibility,
            decls,
            &proc.file,
            &proc.in_arch,
            &call,
            kind,
        );

        let (mut matches, max_score) =
            filter_subprogram_matches(resolver, input, proc, &args, &candidates);
        matches = dedup_subprogram_matches(&matches);
        if matches.len() == 1 {
            continue;
        }
        let has_default_params = matches.iter().any(|m| m.params.len() > args.len());
        if matches.len() > 1 && has_default_params {
            continue;
        }
        if matches.len() > 1 && should_skip_ambiguous_call(resolver, input, proc, &args, &matches) {
            let rule = if call.pkg.is_none() && call.lib.is_none() {
                "ambiguous_unqualified_call"
            } else {
                "ambiguous_subprogram"
            };
            let note = format!(
                "missing argument type info to disambiguate call '{}'",
                name
            );
            skips.record(&proc.file, line, rule, note);
            continue;
        }
        if matches.len() > 1 && candidates_share_signature(resolver, proc, &matches) {
            let rule = if call.pkg.is_none() && call.lib.is_none() {
                "ambiguous_unqualified_call"
            } else {
                "ambiguous_subprogram"
            };
            let note = format!(
                "overloads only differ by subtype constraints for call '{}'",
                name
            );
            skips.record(&proc.file, line, rule, note);
            continue;
        }
        if matches.len() > 1 && max_score <= 1 {
            let rule = if call.pkg.is_none() && call.lib.is_none() {
                "ambiguous_unqualified_call"
            } else {
                "ambiguous_subprogram"
            };
            let note = format!(
                "weak type info for call '{}' ({} candidates)",
                name,
                matches.len()
            );
            skips.record(&proc.file, line, rule, note);
            continue;
        }
        let is_unqualified = call.pkg.is_none() && call.lib.is_none();
        if matches.is_empty() {
            if is_unqualified {
                if proc
                    .variables
                    .iter()
                    .any(|v| v.name.eq_ignore_ascii_case(&call.name))
                {
                    continue;
                }
                let proc_arch = base_arch_name(&proc.in_arch);
                if resolver
                    .resolve_constant_type(&proc.file, proc_arch, &call.name)
                    .is_some()
                {
                    continue;
                }
                if resolver
                    .resolve_type(&proc.file, proc_arch, &call.name)
                    .is_some()
                {
                    continue;
                }
                if is_generate_loop_var(input, &proc.file, proc_arch, &call.name) {
                    continue;
                }
                if is_reserved_identifier(&call.name) {
                    continue;
                }
                if find_signal_type(input, &proc.file, &proc.in_arch, &call.name).is_some()
                    || find_port_type(input, &proc.file, &proc.in_arch, &call.name).is_some()
                    || find_generic_type(input, &proc.file, &proc.in_arch, &call.name).is_some()
                {
                    continue;
                }
                if let Some(pkg) = standard_subprogram_package(&call.name) {
                    if let Some(vis) = visibility.get(&proc.file) {
                        if standard_package_visible(vis, pkg) {
                            continue;
                        }
                    }
                }
            }
            if is_unqualified {
                let mut msg = format!(
                    "{} call '{}' has no matching declaration",
                    kind.as_str(),
                    name
                );
                let file_lib = file_libs
                    .get(&proc.file)
                    .cloned()
                    .unwrap_or_else(|| "work".to_string());
                if let Some(vis) = visibility.get(&proc.file) {
                    let visible = format_visible_packages(vis, 6);
                    if !visible.is_empty() {
                        msg.push_str(&format!(". Visible packages: {}", visible));
                    }
                }
                let all_named = find_all_named_subprograms(decls, kind, &call.name);
                if !all_named.is_empty() {
                    msg.push_str(&format!(
                        ". Found declarations in {}. Add `use <lib>.<pkg>.all` or qualify the call.",
                        summarize_subprograms(&all_named, 4)
                    ));
                } else {
                    msg.push_str(". Action: add a use clause or qualify the call.");
                }
                msg.push_str(&format!(
                    " (file library: {}; work => {})",
                    file_lib, file_lib
                ));
                out.push(Violation {
                    rule: "unresolved_unqualified_call".to_string(),
                    severity: call_severity(input, proc, "error"),
                    file: proc.file.clone(),
                    line,
                    message: msg,
                });
            } else {
                let mut msg = format!(
                    "{} call '{}' has no matching declaration",
                    kind.as_str(),
                    name
                );
                let file_lib = file_libs
                    .get(&proc.file)
                    .cloned()
                    .unwrap_or_else(|| "work".to_string());
                if let Some(lib) = &call.lib {
                    if lib.eq_ignore_ascii_case("work") {
                        msg.push_str(&format!(" (work => {})", file_lib));
                    }
                }
                let all_named = find_all_named_subprograms(decls, kind, &call.name);
                if !all_named.is_empty() {
                    msg.push_str(&format!(
                        ". Available: {}",
                        summarize_subprograms(&all_named, 4)
                    ));
                }
                msg.push_str(". Action: verify library/package mapping or qualify the call");
                out.push(Violation {
                    rule: "unresolved_subprogram".to_string(),
                    severity: call_severity(input, proc, "error"),
                    file: proc.file.clone(),
                    line,
                    message: msg,
                });
            }
        } else if is_unqualified {
            if let Some(pkg) = standard_subprogram_package(&call.name) {
                if let Some(vis) = visibility.get(&proc.file) {
                    if standard_package_visible(vis, pkg) {
                        continue;
                    }
                }
            }
            let mut msg = format!(
                "{} call '{}' matches multiple declarations",
                kind.as_str(),
                name
            );
            msg.push_str(&format!(
                ". Candidates: {}. Qualify the call or remove a conflicting use clause.",
                summarize_subprograms(&matches, 4)
            ));
            out.push(Violation {
                rule: "ambiguous_unqualified_call".to_string(),
                severity: call_severity(input, proc, "error"),
                file: proc.file.clone(),
                line,
                message: msg,
            });
        } else {
            let mut msg = format!(
                "{} call '{}' matches multiple declarations",
                kind.as_str(),
                name
            );
            msg.push_str(&format!(
                ". Candidates: {}. Qualify the call or tighten use clauses.",
                summarize_subprograms(&matches, 4)
            ));
            out.push(Violation {
                rule: "ambiguous_subprogram".to_string(),
                severity: call_severity(input, proc, "error"),
                file: proc.file.clone(),
                line,
                message: msg,
            });
        }
    }

    out.extend(skips.into_violations());
    out
}

fn select_subprogram_candidates(
    input: &Input,
    visibility: &HashMap<String, Visibility>,
    decls: &[SubprogramDecl],
    file: &str,
    arch: &str,
    call: &CallName,
    kind: CallKind,
) -> Vec<SubprogramDecl> {
    let mut out = Vec::new();
    let vis = visibility.get(file);
    let arch_entity = architecture_entity_name(input, file, arch);
    let base_arch = base_arch_name(arch);

    for decl in decls {
        if decl.kind != kind.as_decl_kind() {
            continue;
        }
        if !decl.name.eq_ignore_ascii_case(&call.name) {
            continue;
        }
        if let Some(pkg) = &call.pkg {
            if !decl.in_package.eq_ignore_ascii_case(pkg) {
                continue;
            }
            if let Some(lib) = &call.lib {
                let mut lib_name = lib.to_string();
                if lib_name.eq_ignore_ascii_case("work") {
                    if let Some(v) = vis {
                        if !v.library.is_empty() {
                            lib_name = v.library.clone();
                        }
                    }
                }
                if decl.library.eq_ignore_ascii_case(&lib_name) {
                    out.push(decl.clone());
                }
                continue;
            }
            if let Some(v) = vis {
                let key = format!(
                    "{}.{}",
                    decl.library.to_ascii_lowercase(),
                    decl.in_package.to_ascii_lowercase()
                );
                if v.visible_packages.contains(&key) {
                    out.push(decl.clone());
                }
            }
            continue;
        }
        if !decl.in_arch.is_empty() {
            if decl.in_arch.eq_ignore_ascii_case(arch)
                || (base_arch != arch && decl.in_arch.eq_ignore_ascii_case(base_arch))
            {
                out.push(decl.clone());
                continue;
            }
            if let Some(ent) = arch_entity.as_ref() {
                if decl.in_arch.eq_ignore_ascii_case(ent) {
                    out.push(decl.clone());
                    continue;
                }
            }
            continue;
        }
        if decl.in_package.is_empty() && decl.in_arch.is_empty() {
            if decl.file == file {
                out.push(decl.clone());
            }
            continue;
        }
        if !decl.in_package.is_empty() {
            if let Some(v) = vis {
                let key = format!(
                    "{}.{}",
                    decl.library.to_ascii_lowercase(),
                    decl.in_package.to_ascii_lowercase()
                );
                if v.visible_packages.contains(&key) {
                    out.push(decl.clone());
                }
            }
        }
    }

    out
}

fn filter_subprogram_matches(
    resolver: &TypeResolver,
    input: &Input,
    proc: &Process,
    args: &[String],
    candidates: &[SubprogramDecl],
) -> (Vec<SubprogramDecl>, i32) {
    let mut scored: Vec<(SubprogramDecl, i32)> = Vec::new();
    for cand in candidates {
        if let Some(score) = subprogram_args_match_score(resolver, input, proc, args, cand) {
            scored.push((cand.clone(), score));
        }
    }
    if scored.is_empty() {
        return (Vec::new(), 0);
    }
    let max_score = scored.iter().map(|(_, score)| *score).max().unwrap_or(0);
    let matches = scored
        .into_iter()
        .filter(|(_, score)| *score == max_score)
        .map(|(cand, _)| cand)
        .collect();
    (matches, max_score)
}

fn candidates_share_signature(
    resolver: &TypeResolver,
    proc: &Process,
    matches: &[SubprogramDecl],
) -> bool {
    if matches.len() < 2 {
        return false;
    }
    let first = subprogram_signature(resolver, proc, &matches[0]);
    matches
        .iter()
        .skip(1)
        .all(|m| subprogram_signature(resolver, proc, m) == first)
}

fn should_skip_ambiguous_call(
    resolver: &TypeResolver,
    input: &Input,
    proc: &Process,
    args: &[String],
    matches: &[SubprogramDecl],
) -> bool {
    if matches.is_empty() {
        return false;
    }
    let param_len = matches[0].params.len();
    if param_len == 0 {
        return false;
    }
    let param_names: Vec<String> = matches[0]
        .params
        .iter()
        .map(|p| p.name.to_ascii_lowercase())
        .collect();
    let mut actual_types: Vec<Option<ResolvedType>> = vec![None; param_len];
    let mut positional_idx = 0usize;
    for arg in args {
        if let Some((formal, actual)) = split_named_arg(arg) {
            if let Some(idx) = param_names
                .iter()
                .position(|name| name.eq_ignore_ascii_case(&formal))
            {
                actual_types[idx] = infer_expr_type_for_call(resolver, input, proc, &actual);
            }
            continue;
        }
        if positional_idx < param_len {
            actual_types[positional_idx] = infer_expr_type_for_call(resolver, input, proc, arg);
            positional_idx += 1;
        }
    }
    let mut signature_sets: Vec<HashSet<String>> = vec![HashSet::new(); param_len];
    for decl in matches {
        for (idx, param) in decl.params.iter().enumerate() {
            signature_sets[idx].insert(subprogram_param_signature(resolver, proc, decl, param));
        }
    }
    for (idx, sigs) in signature_sets.iter().enumerate() {
        if sigs.len() <= 1 {
            continue;
        }
        let actual = actual_types.get(idx).and_then(|t| t.as_ref());
        if actual.is_none() {
            return true;
        }
    }
    false
}

fn subprogram_signature(
    resolver: &TypeResolver,
    proc: &Process,
    decl: &SubprogramDecl,
) -> Vec<String> {
    decl
        .params
        .iter()
        .map(|param| subprogram_param_signature(resolver, proc, decl, param))
        .collect()
}

fn subprogram_param_signature(
    resolver: &TypeResolver,
    proc: &Process,
    decl: &SubprogramDecl,
    param: &SubprogramParam,
) -> String {
    let raw = strip_type_mark(&param.canonical).to_ascii_lowercase();
    if !decl.in_package.is_empty() {
        if let Some(resolved) = resolver.resolve_in_package(
            &decl.library,
            &decl.in_package,
            &raw,
            &decl.file,
            &decl.in_arch,
        ) {
            return resolved.canonical.to_ascii_lowercase();
        }
    }
    if let Some(resolved) = resolver.resolve_type(&decl.file, &decl.in_arch, &raw) {
        return resolved.canonical.to_ascii_lowercase();
    }
    if let Some(resolved) = resolver.resolve_type(&proc.file, &proc.in_arch, &raw) {
        return resolved.canonical.to_ascii_lowercase();
    }
    raw
}

fn subprogram_args_match_score(
    resolver: &TypeResolver,
    input: &Input,
    proc: &Process,
    args: &[String],
    decl: &SubprogramDecl,
) -> Option<i32> {
    let params = &decl.params;
    if args.len() > params.len() {
        if params.len() == 1
            && args.len() > 1
            && !args.iter().any(|arg| split_named_arg(arg).is_some())
            && param_allows_aggregate(resolver, proc, &params[0])
        {
            // Treat multiple actuals as an aggregate for a single array parameter.
            return Some(1);
        }
        return None;
    }
    let mut used = vec![false; params.len()];
    let mut positional_idx = 0usize;
    let mut score = 0i32;

    for arg in args {
        if let Some((formal, actual)) = split_named_arg(arg) {
            let mut matched = false;
            for (idx, param) in params.iter().enumerate() {
                if !param.name.eq_ignore_ascii_case(&formal) {
                    continue;
                }
                if let Some(arg_score) =
                    subprogram_arg_score(resolver, input, proc, decl, &actual, param)
                {
                    used[idx] = true;
                    score += arg_score;
                    matched = true;
                    break;
                }
                return None;
            }
            if !matched {
                return None;
            }
            continue;
        }

        while positional_idx < params.len() && used[positional_idx] {
            positional_idx += 1;
        }
        if positional_idx >= params.len() {
            return None;
        }
        let arg_score =
            subprogram_arg_score(resolver, input, proc, decl, arg, &params[positional_idx])?;
        used[positional_idx] = true;
        score += arg_score;
        positional_idx += 1;
    }

    Some(score)
}

fn param_allows_aggregate(
    resolver: &TypeResolver,
    proc: &Process,
    param: &SubprogramParam,
) -> bool {
    if let Some(resolved) = resolver.resolve_type(&proc.file, &proc.in_arch, &param.canonical) {
        if resolved.kind.eq_ignore_ascii_case("array") {
            return true;
        }
        if resolved.canonical.to_ascii_lowercase().contains("vector") {
            return true;
        }
    }
    let lower = param.canonical.to_ascii_lowercase();
    lower.contains("vector")
}

fn split_named_arg(arg: &str) -> Option<(String, String)> {
    let bytes = arg.as_bytes();
    let mut depth = 0i32;
    let mut in_string = false;
    let mut i = 0usize;
    while i + 1 < bytes.len() {
        match bytes[i] {
            b'"' => {
                in_string = !in_string;
            }
            b'(' if !in_string => {
                depth += 1;
            }
            b')' if !in_string && depth > 0 => {
                depth -= 1;
            }
            _ => {}
        }
        if !in_string && depth == 0 && bytes[i] == b'=' {
            let mut j = i + 1;
            while j < bytes.len() && bytes[j].is_ascii_whitespace() {
                j += 1;
            }
            if j >= bytes.len() || bytes[j] != b'>' {
                i += 1;
                continue;
            }
            let formal = arg[..i].trim();
            let actual = arg[j + 1..].trim();
            if formal.is_empty() || actual.is_empty() {
                return None;
            }
            return Some((formal.to_string(), actual.to_string()));
        }
        i += 1;
    }
    None
}

fn actual_is_open(actual: &str) -> bool {
    let trimmed = actual.trim();
    let cleaned = trimmed.trim_end_matches(|c: char| c == ',' || c == ')' || c == ';');
    if cleaned.is_empty() {
        return true;
    }
    if cleaned.eq_ignore_ascii_case("open") {
        return true;
    }
    if let Some((_, rhs)) = split_named_arg(cleaned) {
        let rhs_clean = rhs
            .trim()
            .trim_end_matches(|c: char| c == ',' || c == ')' || c == ';');
        return rhs_clean.eq_ignore_ascii_case("open");
    }
    false
}

fn subprogram_arg_score(
    resolver: &TypeResolver,
    input: &Input,
    proc: &Process,
    decl: &SubprogramDecl,
    actual: &str,
    param: &SubprogramParam,
) -> Option<i32> {
    let actual_type = infer_expr_type_for_call(resolver, input, proc, actual);
    let actual_is_literal = actual.trim().chars().all(|c| c.is_ascii_digit());
    let actual_token = normalize_literal_token(actual);
    match actual_type {
        Some(ResolvedType {
            canonical, kind, ..
        }) => {
            if kind == "numeric" && param.is_numeric {
                if actual_is_literal {
                    return Some(2);
                }
                if canonical.eq_ignore_ascii_case(&param.canonical) {
                    return Some(2);
                }
                return Some(2);
            }
            if canonical.eq_ignore_ascii_case(&param.canonical) {
                return Some(2);
            }
            let mut resolved_param = if !decl.in_package.is_empty() {
                resolver.resolve_in_package(
                    &decl.library,
                    &decl.in_package,
                    &param.canonical,
                    &decl.file,
                    &decl.in_arch,
                )
            } else {
                resolver.resolve_type(&decl.file, &decl.in_arch, &param.canonical)
            };
            if resolved_param.is_none() {
                resolved_param = resolver.resolve_type(&proc.file, &proc.in_arch, &param.canonical);
            }
            if let Some(resolved_param) = resolved_param {
                if (actual_token == "true" || actual_token == "false")
                    && enum_literal_matches_type(
                        resolver,
                        &proc.file,
                        &proc.in_arch,
                        &resolved_param,
                        &actual_token,
                    )
                {
                    return Some(2);
                }
                if canonical.eq_ignore_ascii_case(&resolved_param.canonical) {
                    return Some(2);
                }
                if logic_vector_compatible(&canonical, &resolved_param.canonical) {
                    return Some(2);
                }
                let param_lower = resolved_param.canonical.to_ascii_lowercase();
                if canonical.eq_ignore_ascii_case("std_logic_vector")
                    && (param_lower.contains("unresolved_unsigned")
                        || param_lower.contains("unresolved_signed"))
                {
                    return Some(1);
                }
                if kind == "numeric" && resolved_param.kind == "numeric" {
                    return Some(2);
                }
            } else if !is_builtin_type(&param.canonical.to_ascii_lowercase()) {
                // Treat unknown/generic parameter types as a weak match.
                return Some(1);
            }
            let actual_builtin = is_builtin_type(&canonical.to_ascii_lowercase());
            let param_builtin = is_builtin_type(&param.canonical.to_ascii_lowercase());
            if !actual_builtin && !param_builtin {
                return Some(1);
            }
            if is_complex_actual(actual) {
                return Some(1);
            }
            None
        }
        None => Some(1),
    }
}

fn is_complex_actual(actual: &str) -> bool {
    let trimmed = actual.trim();
    trimmed.contains('.')
        || trimmed.contains('(')
        || trimmed.contains(')')
        || trimmed.contains('&')
        || trimmed.contains('+')
        || trimmed.contains('-')
        || trimmed.contains('*')
        || trimmed.contains('/')
        || trimmed.contains('>')
        || trimmed.contains('<')
        || trimmed.contains('\'')
}

fn infer_expr_type_for_call(
    resolver: &TypeResolver,
    input: &Input,
    proc: &Process,
    expr: &str,
) -> Option<ResolvedType> {
    let raw = expr.trim();
    if raw.is_empty() {
        return None;
    }
    let lower = raw.to_ascii_lowercase();
    if lower == "true" || lower == "false" {
        return Some(ResolvedType::builtin("boolean".to_string()));
    }
    if lower.chars().all(|c| c.is_ascii_digit()) {
        return Some(ResolvedType::builtin("integer".to_string()));
    }
    if is_bit_string_literal(raw) {
        return Some(ResolvedType::builtin("std_logic_vector".to_string()));
    }
    if lower.starts_with('"') && lower.ends_with('"') && lower.len() >= 2 {
        return Some(ResolvedType::builtin("string".to_string()));
    }
    if lower.starts_with("'") && lower.ends_with("'") && lower.len() == 3 {
        return Some(ResolvedType::builtin("std_logic".to_string()));
    }
    if let Some(attr) = infer_attribute_type(raw) {
        return Some(attr);
    }
    if looks_like_boolean_expr(raw) {
        return Some(ResolvedType::builtin("boolean".to_string()));
    }
    if looks_like_arithmetic_expr(raw) {
        return None;
    }
    resolve_named_reference_type_in_proc(input, resolver, proc, raw)
}

fn is_bit_string_literal(raw: &str) -> bool {
    let trimmed = raw.trim();
    if trimmed.len() < 2 || !trimmed.starts_with('"') || !trimmed.ends_with('"') {
        return false;
    }
    let inner = &trimmed[1..trimmed.len() - 1];
    if inner.is_empty() {
        return false;
    }
    inner.chars().all(|c| {
        matches!(
            c,
            '0' | '1'
                | 'x'
                | 'X'
                | 'z'
                | 'Z'
                | 'u'
                | 'U'
                | 'h'
                | 'H'
                | 'l'
                | 'L'
                | 'w'
                | 'W'
                | '-'
                | '_'
        )
    })
}

fn looks_like_boolean_expr(raw: &str) -> bool {
    let bytes = raw.as_bytes();
    let mut in_string = false;
    for i in 0..bytes.len() {
        match bytes[i] {
            b'"' => {
                in_string = !in_string;
                continue;
            }
            _ => {}
        }
        if in_string {
            continue;
        }
        if bytes[i] == b'=' {
            let prev = if i > 0 { bytes[i - 1] } else { 0 };
            let next = if i + 1 < bytes.len() { bytes[i + 1] } else { 0 };
            if prev == b':' || next == b'>' {
                continue;
            }
            return true;
        }
        if bytes[i] == b'>' {
            if i > 0 && bytes[i - 1] == b'=' {
                continue;
            }
            return true;
        }
        if bytes[i] == b'<' {
            return true;
        }
        if bytes[i] == b'/' && i + 1 < bytes.len() && bytes[i + 1] == b'=' {
            return true;
        }
    }
    let lower = raw.to_ascii_lowercase();
    if lower.contains(" and ")
        || lower.contains(" or ")
        || lower.contains(" xor ")
        || lower.contains(" xnor ")
        || lower.contains(" nand ")
        || lower.contains(" nor ")
        || lower.starts_with("not ")
        || lower.starts_with("not(")
        || lower.contains(" not ")
    {
        return true;
    }
    false
}

fn looks_like_arithmetic_expr(raw: &str) -> bool {
    let bytes = raw.as_bytes();
    let mut in_string = false;
    for i in 0..bytes.len() {
        match bytes[i] {
            b'"' => {
                in_string = !in_string;
                continue;
            }
            _ => {}
        }
        if in_string {
            continue;
        }
        if matches!(bytes[i], b'+' | b'-' | b'*' | b'/') {
            return true;
        }
    }
    false
}

#[derive(Clone)]
struct SubprogramDecl {
    name: String,
    kind: String,
    params: Vec<SubprogramParam>,
    return_type: String,
    in_package: String,
    in_arch: String,
    library: String,
    file: String,
}

#[derive(Clone, Default)]
struct CallName {
    lib: Option<String>,
    pkg: Option<String>,
    name: String,
}

#[derive(Clone)]
struct SubprogramParam {
    name: String,
    canonical: String,
    is_numeric: bool,
}

impl SubprogramDecl {
    fn from_function(func: &FunctionDeclaration, library: &str) -> Self {
        Self {
            name: func.name.clone(),
            kind: "function".to_string(),
            params: func
                .parameters
                .iter()
                .map(|p| SubprogramParam::from_param(p))
                .collect(),
            return_type: func.return_type.clone(),
            in_package: func.in_package.clone(),
            in_arch: func.in_arch.clone(),
            library: library.to_string(),
            file: func.file.clone(),
        }
    }

    fn from_procedure(proc: &ProcedureDeclaration, library: &str) -> Self {
        Self {
            name: proc.name.clone(),
            kind: "procedure".to_string(),
            params: proc
                .parameters
                .iter()
                .map(|p| SubprogramParam::from_param(p))
                .collect(),
            return_type: String::new(),
            in_package: proc.in_package.clone(),
            in_arch: proc.in_arch.clone(),
            library: library.to_string(),
            file: proc.file.clone(),
        }
    }
}

impl SubprogramParam {
    fn from_param(param: &SubprogramParameter) -> Self {
        let canon = strip_type_mark(&param.r#type).to_ascii_lowercase();
        Self {
            name: param.name.clone(),
            canonical: canon.clone(),
            is_numeric: is_numeric_type(&canon),
        }
    }
}

#[derive(Clone, Copy)]
enum CallKind {
    Function,
    Procedure,
}

impl CallKind {
    fn as_str(&self) -> &'static str {
        match self {
            CallKind::Function => "Function",
            CallKind::Procedure => "Procedure",
        }
    }

    fn as_decl_kind(&self) -> &'static str {
        match self {
            CallKind::Function => "function",
            CallKind::Procedure => "procedure",
        }
    }
}

fn parse_call_name(name: &str) -> CallName {
    let parts: Vec<&str> = name.split('.').collect();
    if parts.is_empty() {
        return CallName::default();
    }
    if parts.len() >= 3 {
        return CallName {
            lib: Some(parts[0].to_string()),
            pkg: Some(parts[1].to_string()),
            name: parts[parts.len() - 1].to_string(),
        };
    }
    if parts.len() == 2 {
        return CallName {
            lib: None,
            pkg: Some(parts[0].to_string()),
            name: parts[1].to_string(),
        };
    }
    CallName {
        lib: None,
        pkg: None,
        name: parts[0].to_string(),
    }
}

fn standard_subprogram_package(name: &str) -> Option<&'static str> {
    let lower = name.to_ascii_lowercase();
    if lower.starts_with("vital") {
        return Some("ieee.vital_timing");
    }
	match lower.as_str() {
		"unsigned"
		| "signed"
		| "to_integer"
		| "to_unsigned"
		| "to_signed"
		| "resize"
		| "shift_left"
		| "shift_right"
		| "rotate_left"
		| "rotate_right" => {
			Some("ieee.numeric_std")
		}
		"std_logic_vector"
		| "std_ulogic_vector"
		| "to_stdlogicvector"
		| "to_stdulogicvector"
		| "to_stdlogic"
		| "to_stdulogic"
		| "to_bitvector"
		| "std_match"
		| "to_ux01"
		| "to_x01z" => {
			Some("ieee.std_logic_1164")
		}
        "is_x" | "to_x01" | "to_01" => Some("ieee.std_logic_1164"),
        "round" | "ceil" | "floor" | "uniform" => Some("ieee.math_real"),
        "conv_integer" | "conv_unsigned" | "conv_signed" | "conv_std_logic_vector" => {
            Some("ieee.std_logic_arith")
        }
        "write" | "writeline" | "read" | "readline" | "file_open" | "file_close" => {
            Some("std.textio")
        }
        "finish" | "stop" => Some("std.env"),
        _ => None,
    }
}

fn is_reserved_identifier(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "downto" | "to" | "range" | "others"
    )
}

fn is_generate_loop_var(input: &Input, file: &str, arch: &str, name: &str) -> bool {
    let base_arch = base_arch_name(arch);
    input.generates.iter().any(|gen| {
        gen.file == file
            && gen.in_arch.eq_ignore_ascii_case(base_arch)
            && gen.loop_var.eq_ignore_ascii_case(name)
    })
}

fn standard_package_visible(vis: &Visibility, pkg: &str) -> bool {
    if vis
        .visible_packages
        .iter()
        .any(|visible| visible.eq_ignore_ascii_case(pkg))
    {
        return true;
    }
    if pkg.eq_ignore_ascii_case("std.textio") {
        return vis
            .visible_packages
            .iter()
            .any(|visible| visible.eq_ignore_ascii_case("ieee.std_logic_textio"));
    }
    false
}

fn is_standard_qualified_call(call: &CallName) -> bool {
    let lib = match call.lib.as_ref() {
        Some(l) => l.to_ascii_lowercase(),
        None => return false,
    };
    let pkg = match call.pkg.as_ref() {
        Some(p) => p.to_ascii_lowercase(),
        None => return false,
    };
    if lib == "std" {
        return matches!(pkg.as_str(), "env" | "textio");
    }
    if lib == "ieee" {
        return matches!(
            pkg.as_str(),
            "std_logic_1164" | "numeric_std" | "numeric_std_unsigned"
        );
    }
    false
}

fn is_object_call_prefix(input: &Input, proc: &Process, call: &CallName) -> bool {
    if call.lib.is_some() {
        return false;
    }
    let prefix = match call.pkg.as_ref() {
        Some(p) => p,
        None => return false,
    };
    if proc
        .variables
        .iter()
        .any(|v| v.name.eq_ignore_ascii_case(prefix))
    {
        return true;
    }
    if find_signal_type(input, &proc.file, &proc.in_arch, prefix).is_some() {
        return true;
    }
    if find_port_type(input, &proc.file, &proc.in_arch, prefix).is_some() {
        return true;
    }
    input
        .shared_variables
        .iter()
        .any(|name| name.eq_ignore_ascii_case(prefix))
}

fn file_library_map(input: &Input) -> HashMap<String, String> {
    let mut map = HashMap::new();
    for file in &input.files {
        let lib = if file.library.is_empty() {
            "work".to_string()
        } else {
            file.library.to_ascii_lowercase()
        };
        map.insert(file.path.clone(), lib);
    }
    map
}

fn architecture_entity_name(input: &Input, file: &str, arch: &str) -> Option<String> {
    if arch.trim().is_empty() {
        return None;
    }
    if let Some(found) = input
        .architectures
        .iter()
        .find(|a| a.file == file && a.name.eq_ignore_ascii_case(arch))
    {
        return Some(found.entity_name.clone());
    }
    let base = base_arch_name(arch);
    if base != arch {
        return input
            .architectures
            .iter()
            .find(|a| a.file == file && a.name.eq_ignore_ascii_case(base))
            .map(|a| a.entity_name.clone());
    }
    None
}

fn base_arch_name(arch: &str) -> &str {
    arch.split_once('.').map(|(base, _)| base).unwrap_or(arch)
}

fn split_target(target: &str) -> (Option<String>, String) {
    let lower = target.to_ascii_lowercase();
    let parts: Vec<&str> = lower.split('.').collect();
    if parts.len() >= 2 {
        return (
            Some(parts[0].to_string()),
            parts[parts.len() - 1].to_string(),
        );
    }
    (None, lower)
}

fn base_entity_name(target: &str) -> String {
    let parts: Vec<&str> = target.split('.').collect();
    parts.last().unwrap_or(&target).to_string()
}

fn resolve_named_reference_type(
    input: &Input,
    resolver: &TypeResolver,
    file: &str,
    arch: &str,
    raw: &str,
) -> Option<ResolvedType> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return None;
    }
    if let Some(resolved) = resolver.resolve_qualified_constant_or_enum(file, arch, trimmed) {
        return Some(resolved);
    }
    let (base, mut idx) = split_base_identifier(trimmed)?;
    let base_type = find_signal_type(input, file, arch, &base)
        .or_else(|| find_port_type(input, file, arch, &base))
        .or_else(|| find_generic_type(input, file, arch, &base));
    let mut current = match base_type {
        Some(ty) => resolver.resolve_type(file, arch, &ty)?,
        None => {
            if let Some(resolved) = resolver.resolve_constant_type(file, arch, &base) {
                resolved
            } else if let Some(resolved) = resolver.resolve_enum_literal_type(file, arch, &base) {
                return Some(resolved);
            } else {
                return None;
            }
        }
    };

    let bytes = trimmed.as_bytes();
    while idx < bytes.len() {
        while idx < bytes.len() && bytes[idx].is_ascii_whitespace() {
            idx += 1;
        }
        if idx >= bytes.len() {
            break;
        }
        match bytes[idx] {
            b'(' => {
                if let Some(content) = paren_content(trimmed, idx) {
                    if !is_slice_content(&content) {
                        current = resolve_indexed_type(resolver, file, arch, &current)?;
                    }
                } else {
                    current = resolve_indexed_type(resolver, file, arch, &current)?;
                }
                idx = skip_parens(trimmed, idx);
            }
            b'.' => {
                idx += 1;
                let (field, next_idx) = read_identifier(trimmed, idx)?;
                idx = next_idx;
                current = resolve_field_type(resolver, file, arch, &current, &field)?;
            }
            b'\'' => break,
            _ => idx += 1,
        }
    }
    Some(current)
}

fn resolve_named_reference_type_in_proc(
    input: &Input,
    resolver: &TypeResolver,
    proc: &Process,
    raw: &str,
) -> Option<ResolvedType> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return None;
    }
    if let Some(resolved) =
        resolver.resolve_qualified_constant_or_enum(&proc.file, &proc.in_arch, trimmed)
    {
        return Some(resolved);
    }
    let (base, mut idx) = split_base_identifier(trimmed)?;
    let base_type = find_variable_type(proc, &base)
        .or_else(|| find_signal_type(input, &proc.file, &proc.in_arch, &base))
        .or_else(|| find_port_type(input, &proc.file, &proc.in_arch, &base))
        .or_else(|| find_generic_type(input, &proc.file, &proc.in_arch, &base));
    let mut current = match base_type {
        Some(ty) => resolver.resolve_type(&proc.file, &proc.in_arch, &ty)?,
        None => {
            if let Some(resolved) = resolver.resolve_constant_type(&proc.file, &proc.in_arch, &base)
            {
                resolved
            } else if let Some(resolved) =
                resolver.resolve_enum_literal_type(&proc.file, &proc.in_arch, &base)
            {
                return Some(resolved);
            } else {
                return None;
            }
        }
    };

    let bytes = trimmed.as_bytes();
    while idx < bytes.len() {
        while idx < bytes.len() && bytes[idx].is_ascii_whitespace() {
            idx += 1;
        }
        if idx >= bytes.len() {
            break;
        }
        match bytes[idx] {
            b'(' => {
                if let Some(content) = paren_content(trimmed, idx) {
                    if !is_slice_content(&content) {
                        current =
                            resolve_indexed_type(resolver, &proc.file, &proc.in_arch, &current)?;
                    }
                } else {
                    current = resolve_indexed_type(resolver, &proc.file, &proc.in_arch, &current)?;
                }
                idx = skip_parens(trimmed, idx);
            }
            b'.' => {
                idx += 1;
                let (field, next_idx) = read_identifier(trimmed, idx)?;
                idx = next_idx;
                current =
                    resolve_field_type(resolver, &proc.file, &proc.in_arch, &current, &field)?;
            }
            b'\'' => break,
            _ => idx += 1,
        }
    }
    Some(current)
}

fn resolve_indexed_type(
    resolver: &TypeResolver,
    file: &str,
    arch: &str,
    ty: &ResolvedType,
) -> Option<ResolvedType> {
    if ty.kind.eq_ignore_ascii_case("array") {
        if let Some(elem) = ty.element_type.as_ref() {
            return resolver.resolve_type(file, arch, elem);
        }
    }
    let canon = ty.canonical.to_ascii_lowercase();
    if canon.contains("std_logic_vector") || canon.contains("signed") || canon.contains("unsigned")
    {
        return Some(ResolvedType::builtin("std_logic".to_string()));
    }
    if canon.contains("std_ulogic_vector") {
        return Some(ResolvedType::builtin("std_ulogic".to_string()));
    }
    None
}

fn resolve_field_type(
    resolver: &TypeResolver,
    file: &str,
    arch: &str,
    ty: &ResolvedType,
    field: &str,
) -> Option<ResolvedType> {
    if !ty.kind.eq_ignore_ascii_case("record") {
        return None;
    }
    for f in &ty.fields {
        if f.name.eq_ignore_ascii_case(field) {
            return resolver.resolve_type(file, arch, &f.r#type);
        }
    }
    None
}

fn split_base_identifier(raw: &str) -> Option<(String, usize)> {
    let bytes = raw.as_bytes();
    let mut idx = 0;
    while idx < bytes.len() && bytes[idx].is_ascii_whitespace() {
        idx += 1;
    }
    if idx >= bytes.len() || !is_ident_start(bytes[idx]) {
        return None;
    }
    let start = idx;
    idx += 1;
    while idx < bytes.len() && is_ident_part(bytes[idx]) {
        idx += 1;
    }
    Some((raw[start..idx].to_string(), idx))
}

fn read_identifier(raw: &str, start: usize) -> Option<(String, usize)> {
    let bytes = raw.as_bytes();
    let mut idx = start;
    while idx < bytes.len() && bytes[idx].is_ascii_whitespace() {
        idx += 1;
    }
    if idx >= bytes.len() || !is_ident_start(bytes[idx]) {
        return None;
    }
    let begin = idx;
    idx += 1;
    while idx < bytes.len() && is_ident_part(bytes[idx]) {
        idx += 1;
    }
    Some((raw[begin..idx].to_string(), idx))
}

fn skip_parens(raw: &str, start: usize) -> usize {
    let bytes = raw.as_bytes();
    let mut depth = 0;
    let mut idx = start;
    while idx < bytes.len() {
        match bytes[idx] {
            b'(' => depth += 1,
            b')' => {
                depth -= 1;
                if depth == 0 {
                    return idx + 1;
                }
            }
            _ => {}
        }
        idx += 1;
    }
    bytes.len()
}

fn paren_content(raw: &str, start: usize) -> Option<String> {
    let bytes = raw.as_bytes();
    if start >= bytes.len() || bytes[start] != b'(' {
        return None;
    }
    let mut depth = 0;
    let mut idx = start;
    let mut content_start = None;
    while idx < bytes.len() {
        match bytes[idx] {
            b'(' => {
                depth += 1;
                if depth == 1 {
                    content_start = Some(idx + 1);
                }
            }
            b')' => {
                depth -= 1;
                if depth == 0 {
                    if let Some(begin) = content_start {
                        return Some(raw[begin..idx].to_string());
                    }
                    return Some(String::new());
                }
            }
            _ => {}
        }
        idx += 1;
    }
    None
}

fn is_slice_content(content: &str) -> bool {
    let lower = content.to_ascii_lowercase();
    lower.contains("downto") || lower.contains(" to ") || lower.contains("range")
}

fn is_ident_start(b: u8) -> bool {
    (b as char).is_ascii_alphabetic() || b == b'_'
}

fn is_ident_part(b: u8) -> bool {
    is_ident_start(b) || (b as char).is_ascii_digit()
}

fn base_name(raw: &str) -> String {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return String::new();
    }
    let before_idx = trimmed.split('(').next().unwrap_or(trimmed);
    let first = before_idx.split('.').next().unwrap_or(before_idx);
    first.trim().to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{
        Architecture, Association, ConstantDeclaration, Entity, FileInfo, FunctionCall,
        FunctionDeclaration, GenericDecl, Instance, LibraryClause, Port, ProcedureCall,
        ProcedureDeclaration, Process, Signal, SubprogramParameter, SubtypeDeclaration,
        TypeDeclaration, UseClause, VariableDecl,
    };

    fn file_info(path: &str, library: &str) -> FileInfo {
        FileInfo {
            path: path.to_string(),
            library: library.to_string(),
            is_third_party: false,
        }
    }

    #[test]
    fn resolve_visible_subtype_from_package() {
        let file = "/tmp/axi_pkg.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "lib")];
        input.use_clauses = vec![UseClause {
            items: vec!["work.Pkg.all".to_string()],
            file: file.to_string(),
            line: 1,
        }];
        input.subtypes = vec![SubtypeDeclaration {
            name: "Axi4ProtType".to_string(),
            base_type: "std_logic_vector".to_string(),
            file: file.to_string(),
            in_package: "Pkg".to_string(),
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let resolved = resolver.resolve_type(file, "", "Axi4ProtType");
        assert!(resolved.is_some());
    }

    #[test]
    fn resolve_indexed_array_element_type() {
        let file = "/tmp/rtl.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "lib")];
        input.types = vec![TypeDeclaration {
            name: "ByteArray".to_string(),
            kind: "array".to_string(),
            element_type: "std_logic".to_string(),
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        }];
        input.signals = vec![Signal {
            name: "data".to_string(),
            r#type: "ByteArray".to_string(),
            file: file.to_string(),
            in_entity: "rtl".to_string(),
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let inst = Instance {
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        };
        let resolved = infer_expr_type(&input, &resolver, &inst, "data(0)");
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("std_logic")
        ));
    }

    #[test]
    fn infer_expr_type_conversion_call() {
        let file = "/tmp/conv.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let inst = Instance {
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        };
        let resolved = infer_expr_type(&input, &resolver, &inst, "std_ulogic(clk)");
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("std_ulogic")
        ));
    }

    #[test]
    fn infer_expr_type_numeric_expression() {
        let file = "/tmp/arith.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let inst = Instance {
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        };
        let resolved = infer_expr_type(&input, &resolver, &inst, "32+4+1");
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("integer")
        ));
    }

    #[test]
    fn infer_expr_type_numeric_identifier_expression() {
        let file = "/tmp/arith_ident.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.entities = vec![Entity {
            name: "Top".to_string(),
            file: file.to_string(),
            generics: vec![GenericDecl {
                name: "WIDTH".to_string(),
                r#type: "integer".to_string(),
                in_entity: "Top".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "Top".to_string(),
            file: file.to_string(),
            ..Default::default()
        }];
        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let inst = Instance {
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        };
        let resolved = infer_expr_type(&input, &resolver, &inst, "WIDTH + 2");
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("integer")
        ));
    }

    #[test]
    fn infer_expr_type_function_call_return() {
        let file = "/tmp/fn_return.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.use_clauses = vec![UseClause {
            items: vec!["work.pkg.all".to_string()],
            file: file.to_string(),
            line: 1,
        }];
        input.functions = vec![FunctionDeclaration {
            name: "isum".to_string(),
            return_type: "integer".to_string(),
            in_package: "pkg".to_string(),
            file: file.to_string(),
            ..Default::default()
        }];
        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let inst = Instance {
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        };
        let resolved = infer_expr_type(&input, &resolver, &inst, "isum(FOO)");
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("integer")
        ));
    }

    #[test]
    fn infer_expr_type_qualified_aggregate() {
        let file = "/tmp/agg.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let inst = Instance {
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        };
        let resolved =
            infer_expr_type(&input, &resolver, &inst, "std_logic_vector'(others => '0')");
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("std_logic_vector")
        ));
    }

    #[test]
    fn visibility_inherits_entity_context() {
        let ent_file = "/tmp/entity.vhd";
        let arch_file = "/tmp/arch.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(ent_file, "lib"), file_info(arch_file, "lib")];
        input.entities = vec![Entity {
            name: "Top".to_string(),
            file: ent_file.to_string(),
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "Top".to_string(),
            file: arch_file.to_string(),
            ..Default::default()
        }];
        input.use_clauses = vec![UseClause {
            items: vec!["work.Pkg.all".to_string()],
            file: ent_file.to_string(),
            line: 1,
        }];

        let visibility = build_visibility(&input);
        let arch_vis = visibility.get(arch_file).expect("arch visibility");
        assert!(arch_vis
            .visible_packages
            .iter()
            .any(|pkg| pkg.eq_ignore_ascii_case("lib.pkg")));
    }

    #[test]
    fn visibility_merges_duplicate_entities() {
        let ent_file_a = "/tmp/entity_a.vhd";
        let ent_file_b = "/tmp/entity_b.vhd";
        let arch_file = "/tmp/arch.vhd";
        let mut input = Input::default();
        input.files = vec![
            file_info(ent_file_a, "lib"),
            file_info(ent_file_b, "lib"),
            file_info(arch_file, "lib"),
        ];
        input.entities = vec![
            Entity {
                name: "Top".to_string(),
                file: ent_file_a.to_string(),
                ..Default::default()
            },
            Entity {
                name: "Top".to_string(),
                file: ent_file_b.to_string(),
                ..Default::default()
            },
        ];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "Top".to_string(),
            file: arch_file.to_string(),
            ..Default::default()
        }];
        input.use_clauses = vec![UseClause {
            items: vec!["work.Pkg.all".to_string()],
            file: ent_file_b.to_string(),
            line: 1,
        }];

        let visibility = build_visibility(&input);
        let arch_vis = visibility.get(arch_file).expect("arch visibility");
        assert!(arch_vis
            .visible_packages
            .iter()
            .any(|pkg| pkg.eq_ignore_ascii_case("lib.pkg")));
    }

    #[test]
    fn split_named_arg_ignores_aggregate_assoc() {
        let aggregate = "GetAlertCount = AlertCountType'(FAILURE => 1, ERROR => 0, WARNING => 0)";
        assert!(split_named_arg(aggregate).is_none());

        let named = "Formal => Actual";
        assert_eq!(
            split_named_arg(named),
            Some(("Formal".to_string(), "Actual".to_string()))
        );

        let spaced = "Formal = > Actual";
        assert_eq!(
            split_named_arg(spaced),
            Some(("Formal".to_string(), "Actual".to_string()))
        );
    }

    #[test]
    fn binding_resolves_visible_library() {
        let inst_file = "/tmp/inst.vhd";
        let ent_file = "/tmp/ent.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(inst_file, "lib_a"), file_info(ent_file, "lib_b")];
        input.entities = vec![Entity {
            name: "OtherEnt".to_string(),
            file: ent_file.to_string(),
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "OtherEnt".to_string(),
            file: ent_file.to_string(),
            ..Default::default()
        }];
        input.instances = vec![Instance {
            name: "u0".to_string(),
            target: "OtherEnt".to_string(),
            file: inst_file.to_string(),
            in_arch: "TopArch".to_string(),
            ..Default::default()
        }];
        input.library_clauses = vec![LibraryClause {
            libraries: vec!["lib_b".to_string()],
            file: inst_file.to_string(),
            line: 1,
        }];

        let visibility = build_visibility(&input);
        let bindings = build_binding_table(&input, &visibility);
        let violations = instance_binding_violations(&input, &bindings);
        assert!(violations.is_empty());
    }

    #[test]
    fn binding_resolves_work_library() {
        let inst_file = "/tmp/inst_work.vhd";
        let ent_file = "/tmp/ent_work.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(inst_file, "lib_x"), file_info(ent_file, "lib_x")];
        input.entities = vec![Entity {
            name: "WorkEnt".to_string(),
            file: ent_file.to_string(),
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "WorkEnt".to_string(),
            file: ent_file.to_string(),
            ..Default::default()
        }];
        input.instances = vec![Instance {
            name: "u_work".to_string(),
            target: "work.WorkEnt".to_string(),
            file: inst_file.to_string(),
            in_arch: "TopArch".to_string(),
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let bindings = build_binding_table(&input, &visibility);
        let violations = instance_binding_violations(&input, &bindings);
        assert!(violations.is_empty());
    }

    #[test]
    fn resolve_call_uses_process_variable_types() {
        let file = "/tmp/proc_vars.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "lib")];
        input.processes = vec![Process {
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            variables: vec![VariableDecl {
                name: "v".to_string(),
                r#type: "std_logic_vector(7 downto 0)".to_string(),
                line: 1,
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let proc = &input.processes[0];
        let resolved = infer_expr_type_for_call(&resolver, &input, proc, "v(0)");
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("std_logic")
        ));
    }

    #[test]
    fn resolve_slice_keeps_vector_type() {
        let file = "/tmp/slice.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "lib")];
        input.signals = vec![Signal {
            name: "data".to_string(),
            r#type: "std_logic_vector(7 downto 0)".to_string(),
            file: file.to_string(),
            in_entity: "rtl".to_string(),
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let resolved =
            resolve_named_reference_type(&input, &resolver, file, "rtl", "data(7 downto 0)");
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("std_logic_vector")
        ));
    }

    #[test]
    fn resolve_record_field_indexed_constant() {
        let file = "/tmp/configs.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "lib")];
        input.types = vec![
            TypeDeclaration {
                name: "bool_t".to_string(),
                kind: "array".to_string(),
                element_type: "boolean".to_string(),
                file: file.to_string(),
                in_arch: "rtl".to_string(),
                ..Default::default()
            },
            TypeDeclaration {
                name: "configs_t".to_string(),
                kind: "record".to_string(),
                fields: vec![RecordField {
                    name: "riscv_c".to_string(),
                    r#type: "bool_t".to_string(),
                }],
                file: file.to_string(),
                in_arch: "rtl".to_string(),
                ..Default::default()
            },
        ];
        input.constant_decls = vec![ConstantDeclaration {
            name: "configs_c".to_string(),
            r#type: "configs_t".to_string(),
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let resolved = resolve_named_reference_type(
            &input,
            &resolver,
            file,
            "rtl",
            "configs_c.riscv_c(CONFIG)",
        );
        assert!(matches!(
            resolved,
            Some(ResolvedType { canonical, .. }) if canonical.eq_ignore_ascii_case("boolean")
        ));
    }

    #[test]
    fn formal_access_detects_element_and_slice() {
        assert!(matches!(
            formal_access_kind(Some("Input(0)")),
            FormalAccessKind::Element
        ));
        assert!(matches!(
            formal_access_kind(Some("Input(7 downto 0)")),
            FormalAccessKind::Slice
        ));
        assert!(matches!(
            formal_access_kind(Some("Input")),
            FormalAccessKind::None
        ));
    }

    #[test]
    fn port_indexed_formal_uses_element_type() {
        let top_file = "/tmp/top.vhd";
        let ent_file = "/tmp/child.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(top_file, "lib"), file_info(ent_file, "lib")];
        input.entities = vec![Entity {
            name: "Child".to_string(),
            file: ent_file.to_string(),
            ports: vec![Port {
                name: "Input".to_string(),
                direction: "in".to_string(),
                r#type: "std_logic_vector(0 downto 0)".to_string(),
                in_entity: "Child".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "Child".to_string(),
            file: ent_file.to_string(),
            ..Default::default()
        }];
        input.signals = vec![Signal {
            name: "sig".to_string(),
            r#type: "std_logic".to_string(),
            file: top_file.to_string(),
            in_entity: "Top".to_string(),
            ..Default::default()
        }];
        input.instances = vec![Instance {
            name: "u0".to_string(),
            target: "Child".to_string(),
            file: top_file.to_string(),
            in_arch: "Top".to_string(),
            associations: vec![Association {
                kind: "port".to_string(),
                formal: "Input(0)".to_string(),
                actual: "sig".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let bindings = build_binding_table(&input, &visibility);
        let violations = port_map_conformance(&input, &resolver, &bindings);
        assert!(violations.iter().all(|v| v.rule != "port_type_mismatch"));
    }

    #[test]
    fn open_output_port_is_allowed() {
        let top_file = "/tmp/top_open.vhd";
        let ent_file = "/tmp/child_open.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(top_file, "lib"), file_info(ent_file, "lib")];
        input.entities = vec![Entity {
            name: "Child".to_string(),
            file: ent_file.to_string(),
            ports: vec![Port {
                name: "DO1".to_string(),
                direction: "out".to_string(),
                r#type: "std_logic".to_string(),
                in_entity: "Child".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "Child".to_string(),
            file: ent_file.to_string(),
            ..Default::default()
        }];
        input.instances = vec![Instance {
            name: "u0".to_string(),
            target: "Child".to_string(),
            file: top_file.to_string(),
            in_arch: "Top".to_string(),
            associations: vec![Association {
                kind: "port".to_string(),
                formal: "DO1".to_string(),
                actual: "DO1=>open".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let bindings = build_binding_table(&input, &visibility);
        let violations = port_map_conformance(&input, &resolver, &bindings);
        assert!(violations
            .iter()
            .all(|v| v.rule != "missing_port_connection"));
        assert!(violations.iter().all(|v| v.rule != "port_type_mismatch"));
    }

    #[test]
    fn array_std_logic_compatible_with_vector() {
        let array_ty = ResolvedType {
            canonical: "poc.vectors.t_slm".to_string(),
            kind: "array".to_string(),
            constraint: None,
            element_type: Some("std_logic".to_string()),
            fields: Vec::new(),
        };
        let vec_ty = ResolvedType::builtin("std_logic_vector".to_string());
        let input = Input::default();
        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        assert!(types_compatible(&resolver, &array_ty, &vec_ty));
    }

    #[test]
    fn standard_qualified_call_is_skipped() {
        let file = "/tmp/proc.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.processes = vec![Process {
            file: file.to_string(),
            line: 1,
            procedure_calls: vec![ProcedureCall {
                name: "std".to_string(),
                full_name: "std.env.stop".to_string(),
                line: 1,
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Procedure,
        );
        assert!(violations.is_empty());
    }

    #[test]
    fn builtin_type_conversion_call_is_skipped() {
        let file = "/tmp/proc_builtin.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.processes = vec![Process {
            file: file.to_string(),
            line: 1,
            function_calls: vec![FunctionCall {
                name: "REAL".to_string(),
                args: vec!["1".to_string()],
                line: 1,
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Function,
        );
        assert!(violations.is_empty());
    }

    #[test]
    fn unqualified_call_matching_port_is_skipped() {
        let file = "/tmp/proc_port_call.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.entities = vec![Entity {
            name: "dut".to_string(),
            file: file.to_string(),
            ports: vec![Port {
                name: "BOOT_ADDR".to_string(),
                direction: "in".to_string(),
                r#type: "std_logic_vector(31 downto 0)".to_string(),
                in_entity: "dut".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        }];
        input.ports = vec![Port {
            name: "BOOT_ADDR".to_string(),
            direction: "in".to_string(),
            r#type: "std_logic_vector(31 downto 0)".to_string(),
            in_entity: "dut".to_string(),
            file: file.to_string(),
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "dut".to_string(),
            file: file.to_string(),
            ..Default::default()
        }];
        input.processes = vec![Process {
            file: file.to_string(),
            line: 10,
            in_arch: "rtl".to_string(),
            function_calls: vec![FunctionCall {
                name: "BOOT_ADDR".to_string(),
                args: vec!["31 downto 2".to_string()],
                line: 10,
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Function,
        );
        assert!(violations
            .iter()
            .all(|v| v.rule != "unresolved_unqualified_call"));
    }

    #[test]
    fn unqualified_call_matching_generic_is_skipped() {
        let file = "/tmp/proc_generic_call.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.entities = vec![Entity {
            name: "dut".to_string(),
            file: file.to_string(),
            generics: vec![GenericDecl {
                name: "BOOT_ADDR".to_string(),
                r#type: "std_logic_vector(31 downto 0)".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "dut".to_string(),
            file: file.to_string(),
            ..Default::default()
        }];
        input.processes = vec![Process {
            file: file.to_string(),
            line: 10,
            in_arch: "rtl".to_string(),
            function_calls: vec![FunctionCall {
                name: "BOOT_ADDR".to_string(),
                args: vec!["31 downto 2".to_string()],
                line: 10,
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Function,
        );
        assert!(violations
            .iter()
            .all(|v| v.rule != "unresolved_unqualified_call"));
    }

    #[test]
    fn nested_generate_scope_sees_arch_subprograms() {
        let file = "/tmp/proc_gen_call.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.entities = vec![Entity {
            name: "top".to_string(),
            file: file.to_string(),
            ..Default::default()
        }];
        input.architectures = vec![Architecture {
            name: "rtl".to_string(),
            entity_name: "top".to_string(),
            file: file.to_string(),
            ..Default::default()
        }];
        input.functions = vec![FunctionDeclaration {
            name: "f".to_string(),
            return_type: "integer".to_string(),
            parameters: vec![SubprogramParameter {
                name: "a".to_string(),
                r#type: "integer".to_string(),
                ..Default::default()
            }],
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        }];
        input.processes = vec![Process {
            file: file.to_string(),
            line: 10,
            in_arch: "rtl.gen1".to_string(),
            function_calls: vec![FunctionCall {
                name: "f".to_string(),
                args: vec!["1".to_string()],
                line: 10,
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Function,
        );
        assert!(violations
            .iter()
            .all(|v| v.rule != "unresolved_unqualified_call"));
    }

    #[test]
    fn math_real_round_is_skipped_when_visible() {
        let file = "/tmp/proc_round_call.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.use_clauses = vec![UseClause {
            items: vec!["ieee.math_real.all".to_string()],
            file: file.to_string(),
            line: 1,
        }];
        input.processes = vec![Process {
            file: file.to_string(),
            line: 1,
            function_calls: vec![FunctionCall {
                name: "round".to_string(),
                args: vec!["1.2".to_string()],
                line: 1,
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Function,
        );
        assert!(violations
            .iter()
            .all(|v| v.rule != "unresolved_unqualified_call"));
    }

    #[test]
    fn std_env_finish_is_skipped_when_visible() {
        let file = "/tmp/proc_finish_call.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.use_clauses = vec![UseClause {
            items: vec!["std.env.all".to_string()],
            file: file.to_string(),
            line: 1,
        }];
        input.processes = vec![Process {
            file: file.to_string(),
            line: 1,
            procedure_calls: vec![ProcedureCall {
                name: "finish".to_string(),
                args: vec![],
                line: 1,
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Procedure,
        );
        assert!(violations
            .iter()
            .all(|v| v.rule != "unresolved_unqualified_call"));
    }

    #[test]
    fn boolean_expression_arg_matches_boolean_param() {
        let file = "/tmp/proc_bool.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.procedures = vec![ProcedureDeclaration {
            name: "simAssertion".to_string(),
            parameters: vec![
                SubprogramParameter {
                    name: "cond".to_string(),
                    r#type: "boolean".to_string(),
                    ..Default::default()
                },
                SubprogramParameter {
                    name: "message".to_string(),
                    r#type: "string".to_string(),
                    ..Default::default()
                },
            ],
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        }];
        input.processes = vec![Process {
            file: file.to_string(),
            line: 1,
            in_arch: "rtl".to_string(),
            procedure_calls: vec![ProcedureCall {
                name: "simAssertion".to_string(),
                args: vec!["fullA = '0'".to_string(), "\"msg\"".to_string()],
                line: 1,
                ..Default::default()
            }],
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Procedure,
        );
        assert!(violations.is_empty());
    }

    #[test]
    fn resolve_calls_allows_defaulted_params() {
        let file = "/tmp/defaults.vhd";
        let mut input = Input::default();
        input.files = vec![file_info(file, "work")];
        input.processes = vec![Process {
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            procedure_calls: vec![ProcedureCall {
                name: "DoIt".to_string(),
                line: 1,
                ..Default::default()
            }],
            ..Default::default()
        }];
        input.procedures = vec![ProcedureDeclaration {
            name: "DoIt".to_string(),
            parameters: vec![
                SubprogramParameter {
                    name: "a".to_string(),
                    r#type: "integer".to_string(),
                    ..Default::default()
                },
                SubprogramParameter {
                    name: "b".to_string(),
                    r#type: "integer".to_string(),
                    ..Default::default()
                },
            ],
            file: file.to_string(),
            in_arch: "rtl".to_string(),
            ..Default::default()
        }];

        let visibility = build_visibility(&input);
        let resolver = TypeResolver::new(&input, &visibility, &[]);
        let decls = build_subprogram_decls(&input, &[]);
        let file_libs = file_library_map(&input);
        let proc = &input.processes[0];
        let violations = resolve_calls(
            &input,
            &resolver,
            &visibility,
            &file_libs,
            proc,
            &decls,
            CallKind::Procedure,
        );
        assert!(violations.is_empty());
    }
}

fn find_signal_type(input: &Input, file: &str, arch: &str, name: &str) -> Option<String> {
    let entity = if arch.is_empty() {
        None
    } else {
        arch_to_entity(input, file, arch)
    };
    let base_arch = base_arch_name(arch);
    let file_libs = file_library_map(input);
    let lib = file_libs
        .get(file)
        .cloned()
        .unwrap_or_else(|| "work".to_string())
        .to_ascii_lowercase();
    let mut entity_files: Vec<String> = entity
        .as_ref()
        .map(|ent| {
            input
                .entities
                .iter()
                .filter(|e| {
                    e.name.eq_ignore_ascii_case(ent)
                        && file_libs
                            .get(&e.file)
                            .map(|l| l.eq_ignore_ascii_case(&lib))
                            .unwrap_or(false)
                })
                .map(|e| e.file.clone())
                .collect()
        })
        .unwrap_or_default();
    entity_files.sort();
    entity_files.dedup();

    let mut same_file_types = Vec::new();
    let mut entity_types = Vec::new();
    for sig in &input.signals {
        if !sig.name.eq_ignore_ascii_case(name) {
            continue;
        }
        let same_file = sig.file == file;
        if !same_file && !entity_files.iter().any(|f| f == &sig.file) {
            continue;
        }
        if !arch.is_empty() {
            let arch_match = sig.in_entity.eq_ignore_ascii_case(arch)
                || (base_arch != arch && sig.in_entity.eq_ignore_ascii_case(base_arch));
            let entity_match = entity
                .as_ref()
                .map(|ent| sig.in_entity.eq_ignore_ascii_case(ent))
                .unwrap_or(false);
            if !arch_match && !entity_match && !sig.in_entity.is_empty() {
                continue;
            }
        }
        if same_file {
            same_file_types.push(sig.r#type.clone());
        } else {
            entity_types.push(sig.r#type.clone());
        }
    }

    unique_type(&same_file_types).or_else(|| unique_type(&entity_types))
}

fn find_variable_type(proc: &Process, name: &str) -> Option<String> {
    for var in &proc.variables {
        if var.name.eq_ignore_ascii_case(name) {
            return Some(var.r#type.clone());
        }
    }
    None
}

fn find_port_type(input: &Input, file: &str, arch: &str, name: &str) -> Option<String> {
    let entity = arch_to_entity(input, file, arch)?;
    let file_libs = file_library_map(input);
    let lib = file_libs
        .get(file)
        .cloned()
        .unwrap_or_else(|| "work".to_string())
        .to_ascii_lowercase();
    let mut entity_files: Vec<String> = input
        .entities
        .iter()
        .filter(|ent| {
            ent.name.eq_ignore_ascii_case(&entity)
                && file_libs
                    .get(&ent.file)
                    .map(|l| l.eq_ignore_ascii_case(&lib))
                    .unwrap_or(false)
        })
        .map(|ent| ent.file.clone())
        .collect();
    entity_files.sort();
    entity_files.dedup();

    let mut matches = Vec::new();
    for port in &input.ports {
        if !port.name.eq_ignore_ascii_case(name) {
            continue;
        }
        if !port.in_entity.eq_ignore_ascii_case(&entity) {
            continue;
        }
        if !port.file.is_empty() {
            if !entity_files.is_empty() && !entity_files.iter().any(|f| f == &port.file) {
                continue;
            }
        }
        matches.push(port.r#type.clone());
    }
    unique_type(&matches)
}

fn find_generic_type(input: &Input, file: &str, arch: &str, name: &str) -> Option<String> {
    let entity = arch_to_entity(input, file, arch)?;
    let file_libs = file_library_map(input);
    let lib = file_libs
        .get(file)
        .cloned()
        .unwrap_or_else(|| "work".to_string())
        .to_ascii_lowercase();
    let mut entity_files: Vec<String> = input
        .entities
        .iter()
        .filter(|ent| {
            ent.name.eq_ignore_ascii_case(&entity)
                && file_libs
                    .get(&ent.file)
                    .map(|l| l.eq_ignore_ascii_case(&lib))
                    .unwrap_or(false)
        })
        .map(|ent| ent.file.clone())
        .collect();
    entity_files.sort();
    entity_files.dedup();

    let mut matches = Vec::new();
    for ent in &input.entities {
        if !ent.name.eq_ignore_ascii_case(&entity) {
            continue;
        }
        if !ent.file.is_empty() {
            if !entity_files.is_empty() && !entity_files.iter().any(|f| f == &ent.file) {
                continue;
            }
        }
        for gen in &ent.generics {
            if gen.name.eq_ignore_ascii_case(name) {
                matches.push(gen.r#type.clone());
            }
        }
    }
    unique_type(&matches)
}

fn unique_type(types: &[String]) -> Option<String> {
    if types.is_empty() {
        return None;
    }
    let mut unique = HashSet::new();
    let mut first = None;
    for ty in types {
        if first.is_none() {
            first = Some(ty.clone());
        }
        unique.insert(ty.trim().to_ascii_lowercase());
    }
    if unique.len() == 1 {
        return first;
    }
    None
}

fn parent_port_map(input: &Input, inst: &Instance) -> HashMap<String, String> {
    let mut map = HashMap::new();
    let entity = match arch_to_entity(input, &inst.file, &inst.in_arch) {
        Some(e) => e,
        None => return map,
    };
    let file_libs = file_library_map(input);
    let lib = file_libs
        .get(&inst.file)
        .cloned()
        .unwrap_or_else(|| "work".to_string())
        .to_ascii_lowercase();
    let mut entity_files: Vec<String> = input
        .entities
        .iter()
        .filter(|ent| {
            ent.name.eq_ignore_ascii_case(&entity)
                && file_libs
                    .get(&ent.file)
                    .map(|l| l.eq_ignore_ascii_case(&lib))
                    .unwrap_or(false)
        })
        .map(|ent| ent.file.clone())
        .collect();
    entity_files.sort();
    entity_files.dedup();
    for port in &input.ports {
        if port.in_entity.eq_ignore_ascii_case(&entity) {
            if !port.file.is_empty()
                && !entity_files.is_empty()
                && !entity_files.iter().any(|f| f == &port.file)
            {
                continue;
            }
            map.insert(port.name.to_ascii_lowercase(), port.direction.clone());
        }
    }
    map
}

fn arch_to_entity(input: &Input, file: &str, arch: &str) -> Option<String> {
    for a in &input.architectures {
        if a.file == file && a.name.eq_ignore_ascii_case(arch) {
            return Some(a.entity_name.clone());
        }
    }
    let base = base_arch_name(arch);
    if base != arch {
        for a in &input.architectures {
            if a.file == file && a.name.eq_ignore_ascii_case(base) {
                return Some(a.entity_name.clone());
            }
        }
    }
    None
}

fn entity_file_for_name(input: &Input, entity: &str) -> Option<String> {
    input
        .entities
        .iter()
        .find(|ent| ent.name.eq_ignore_ascii_case(entity))
        .map(|ent| ent.file.clone())
}
