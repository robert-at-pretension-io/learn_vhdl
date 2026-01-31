#!/usr/bin/env python3
import json
import sys

from pyDatalog import pyDatalog

pyDatalog.create_terms("entity, uses_pkg, entity_uses_pkg, Ent, File, Pkg, E, F, P")


def main():
    if len(sys.argv) < 2:
        print("Usage: datalog_demo.py <facts.json>", file=sys.stderr)
        return 1

    with open(sys.argv[1], "r", encoding="utf-8") as f:
        facts = json.load(f)

    for row in facts.get("entities", []):
        +entity(row["name"], row["file"])

    for row in facts.get("use_clauses", []):
        item = row["item"]
        if ".all" in item:
            pkg_name = item.split(".all")[0]
        else:
            pkg_name = item
        +uses_pkg(row["file"], pkg_name)

    # Example derived relation: entity uses package if its file uses the package.
    entity_uses_pkg(E, P) <= entity(E, F) & uses_pkg(F, P)

    results = entity_uses_pkg(Ent, Pkg)
    print("Derived entity_uses_pkg facts:")
    for ent, pkg in results:
        print(f"  {ent} -> {pkg}")

    print(f"\nCounts: entities={len(facts.get('entities', []))} uses={len(facts.get('use_clauses', []))}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
