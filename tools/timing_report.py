#!/usr/bin/env python3
import argparse
import json
import math
from collections import defaultdict


def read_events(path):
    events = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            events.append(json.loads(line))
    return events


def fmt_ms(ms):
    if ms >= 1000:
        return f"{ms / 1000.0:.2f}s"
    if ms >= 1:
        return f"{ms:.2f}ms"
    return f"{ms * 1000.0:.0f}us"


def bar(value, total, width=40):
    if total <= 0:
        return ""
    n = int(math.floor((value / total) * width))
    return "#" * max(n, 0)


def render_report(events, top_files=15):
    stage_events = [e for e in events if e.get("kind") == "stage"]
    file_events = [e for e in events if e.get("kind") == "file"]
    if not events:
        return "No timing events found."

    total_event = next((e for e in stage_events if e.get("phase") == "total"), None)
    total_ms = total_event["duration_ms"] if total_event else max(e.get("end_ms", 0) for e in events)

    lines = []
    lines.append("# Timing Report")
    lines.append("")
    lines.append("## Stage Summary")
    lines.append("")
    if not stage_events:
        lines.append("_No stage events found._")
    else:
        stage_events_sorted = sorted(stage_events, key=lambda e: e.get("start_ms", 0))
        for e in stage_events_sorted:
            status = f" ({e['status']})" if e.get("status") else ""
            lines.append(
                f"- {e['phase']}{status}: {fmt_ms(e['duration_ms'])} {bar(e['duration_ms'], total_ms)}"
            )

    lines.append("")
    lines.append("## Timeline")
    lines.append("")
    if not stage_events:
        lines.append("_No stage events found._")
    else:
        for e in sorted(stage_events, key=lambda e: e.get("start_ms", 0)):
            status = f" ({e['status']})" if e.get("status") else ""
            lines.append(
                f"- {fmt_ms(e['start_ms'])} → {fmt_ms(e['end_ms'])}: {e['phase']}{status}"
            )

    if file_events:
        lines.append("")
        lines.append("## Slowest Extraction Files")
        lines.append("")
        top = sorted(file_events, key=lambda e: e.get("duration_ms", 0), reverse=True)[:top_files]
        for e in top:
            status = f" ({e['status']})" if e.get("status") else ""
            lines.append(
                f"- {e['file']}{status}: {fmt_ms(e['duration_ms'])} {bar(e['duration_ms'], total_ms)}"
            )

        phase_totals = defaultdict(float)
        for e in file_events:
            phase_totals[e.get("phase", "unknown")] += e.get("duration_ms", 0.0)
        if phase_totals:
            lines.append("")
            lines.append("## File Totals by Phase")
            lines.append("")
            for phase, total in sorted(phase_totals.items(), key=lambda x: x[1], reverse=True):
                lines.append(f"- {phase}: {fmt_ms(total)}")

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="Summarize vhdl-lint timing JSONL")
    parser.add_argument("path", help="Path to timing.jsonl")
    parser.add_argument("--top-files", type=int, default=15, help="Number of slow files to show")
    parser.add_argument("--out", help="Write report to file instead of stdout")
    args = parser.parse_args()

    events = read_events(args.path)
    report = render_report(events, top_files=args.top_files)

    if args.out:
        with open(args.out, "w", encoding="utf-8") as f:
            f.write(report)
            f.write("\n")
    else:
        print(report)


if __name__ == "__main__":
    main()
