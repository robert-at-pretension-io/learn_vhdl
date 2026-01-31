#!/usr/bin/env python3
import json
import os
import subprocess
import sys
from collections import Counter
from pathlib import Path


def run_lint(path: str) -> dict:
    env = os.environ.copy()
    env.setdefault("VHDL_POLICY_DAEMON", "1")
    cmd = ["./vhdl-lint", "-j", path]
    proc = subprocess.run(cmd, capture_output=True, text=True, env=env)
    if proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or proc.stdout.strip())
    return json.loads(proc.stdout)


def violation_key(v: dict) -> str:
    return f"{v.get('rule')}|{v.get('file')}|{v.get('line')}|{v.get('message')}"


def summarize(violations: list) -> dict:
    by_rule = Counter(v.get("rule") for v in violations)
    return {
        "total": len(violations),
        "by_rule": dict(sorted(by_rule.items(), key=lambda kv: (-kv[1], kv[0]))),
    }


def load_json(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def save_json(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as f:
        json.dump(data, f, indent=2, sort_keys=True)


def diff(baseline: dict, current: dict) -> dict:
    base_keys = {violation_key(v) for v in baseline.get("violations", [])}
    cur_keys = {violation_key(v) for v in current.get("violations", [])}
    removed = base_keys - cur_keys
    added = cur_keys - base_keys
    return {
        "removed": sorted(removed),
        "added": sorted(added),
    }


def print_summary(label: str, data: dict) -> None:
    summary = summarize(data.get("violations", []))
    print(f"{label}: {summary['total']} total")
    for rule, count in summary["by_rule"].items():
        print(f"  {rule}: {count}")


def main() -> int:
    if len(sys.argv) < 2:
        print("Usage: tools/fp_loop.py [baseline|diff|last] <path> [state_dir]")
        return 2

    mode = sys.argv[1]
    if mode not in {"baseline", "diff", "last"}:
        path = sys.argv[1]
        mode = "diff"
    else:
        if len(sys.argv) < 3:
            print("Usage: tools/fp_loop.py [baseline|diff|last] <path> [state_dir]")
            return 2
        path = sys.argv[2]

    state_dir = Path(sys.argv[3]) if len(sys.argv) >= 4 else Path(".fp_loop")
    baseline_path = state_dir / "baseline.json"
    last_path = state_dir / "last.json"

    current = run_lint(path)
    if mode == "baseline" or not baseline_path.exists():
        save_json(baseline_path, current)
        save_json(last_path, current)
        print_summary("baseline", current)
        print(f"baseline saved to {baseline_path}")
        return 0

    if mode == "last":
        baseline = load_json(last_path) if last_path.exists() else load_json(baseline_path)
        label = "last"
    else:
        baseline = load_json(baseline_path)
        label = "baseline"

    delta = diff(baseline, current)
    print_summary(label, baseline)
    print_summary("current", current)
    print(f"removed: {len(delta['removed'])}")
    for item in delta["removed"][:50]:
        print(f"  - {item}")
    if len(delta["removed"]) > 50:
        print(f"  ... ({len(delta['removed']) - 50} more)")
    print(f"added: {len(delta['added'])}")
    for item in delta["added"][:50]:
        print(f"  + {item}")
    if len(delta["added"]) > 50:
        print(f"  ... ({len(delta['added']) - 50} more)")

    save_json(last_path, current)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
