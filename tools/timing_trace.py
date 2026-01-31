#!/usr/bin/env python3
import argparse
import json
import os


def read_events(path):
    events = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            events.append(json.loads(line))
    return events


def to_trace_events(events):
    trace = []
    pid = 1
    tid_map = {}
    next_tid = 1

    def tid_for(key):
        nonlocal next_tid
        if key not in tid_map:
            tid_map[key] = next_tid
            trace.append(
                {
                    "name": "thread_name",
                    "ph": "M",
                    "pid": pid,
                    "tid": next_tid,
                    "args": {"name": key},
                }
            )
            next_tid += 1
        return tid_map[key]

    for ev in events:
        phase = ev.get("phase", "unknown")
        kind = ev.get("kind", "unknown")
        status = ev.get("status", "")
        file = ev.get("file", "")
        name = phase
        if kind == "file":
            base = os.path.basename(file)
            name = f"{phase}:{base}"
        if status:
            name = f"{name} ({status})"

        key = f"{kind}:{phase}"
        tid = tid_for(key)
        trace.append(
            {
                "name": name,
                "cat": "vhdl-lint",
                "ph": "X",
                "pid": pid,
                "tid": tid,
                "ts": int(float(ev.get("start_ms", 0)) * 1000.0),
                "dur": int(float(ev.get("duration_ms", 0)) * 1000.0),
                "args": {
                    "kind": kind,
                    "phase": phase,
                    "file": file,
                    "status": status,
                },
            }
        )

    return trace


def main():
    parser = argparse.ArgumentParser(
        description="Convert timing.jsonl into Chrome trace events."
    )
    parser.add_argument("path", help="Path to timing.jsonl")
    parser.add_argument(
        "--out",
        default="timing_trace.json",
        help="Output trace file (default: timing_trace.json)",
    )
    args = parser.parse_args()

    events = read_events(args.path)
    trace = to_trace_events(events)

    with open(args.out, "w", encoding="utf-8") as f:
        json.dump({"traceEvents": trace}, f, indent=2)
        f.write("\n")


if __name__ == "__main__":
    main()
