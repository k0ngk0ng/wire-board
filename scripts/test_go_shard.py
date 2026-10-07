#!/usr/bin/env python3
"""Run an exhaustive, stable shard of Go tests, examples and fuzz seed tests."""

import argparse
import hashlib
import json
import re
import subprocess
import sys


def discover(packages):
    result = subprocess.run(
        ["go", "test", "-json", "-list", ".", *packages],
        capture_output=True, text=True,
    )
    if result.returncode:
        sys.stderr.write(result.stdout + result.stderr)
        raise SystemExit(result.returncode)
    # Go's list output contains only top-level runnable names. Subtests stay
    # with their parent so fixtures, cleanup and t.Run semantics are unchanged.
    names = set()
    for line in result.stdout.splitlines():
        event = json.loads(line)
        if event.get("Action") == "output":
            name = event.get("Output", "").strip()
            if re.fullmatch(r"(?:Test|Example|Fuzz)\w*", name):
                names.add(name)
    if not names:
        raise SystemExit("No runnable Go tests discovered; refusing an empty verification.")
    return sorted(names)


def partition(names, index, count):
    if count < 1 or index < 0 or index >= count:
        raise ValueError("shard index must be between 0 and count - 1")
    # Hash only the name: an identical name in multiple packages must be on
    # the same shard because go test applies -run to every selected package.
    return [name for name in names
            if int.from_bytes(hashlib.sha256(name.encode()).digest()[:8], "big") % count == index]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--index", type=int, required=True)
    parser.add_argument("--count", type=int, required=True)
    parser.add_argument("--timeout", default="20m")
    parser.add_argument("--race", action="store_true")
    parser.add_argument("--list", action="store_true", help="print selected names without running")
    parser.add_argument("packages", nargs="*", default=["./..."])
    args = parser.parse_args()
    try:
        partition([], args.index, args.count)
    except ValueError as error:
        parser.error(str(error))
    names = discover(args.packages)
    selected = partition(names, args.index, args.count)
    if not selected:
        parser.error("this shard has no tests; reduce the shard count")
    print(f"Go shard {args.index + 1}/{args.count}: {len(selected)}/{len(names)} top-level names", flush=True)
    if args.list:
        print("\n".join(selected))
        return 0
    pattern = "^(?:" + "|".join(re.escape(name) for name in selected) + ")$"
    command = ["go", "test", "-count=1", f"-timeout={args.timeout}", "-run", pattern]
    if args.race:
        command.append("-race")
    return subprocess.run([*command, *args.packages]).returncode


if __name__ == "__main__":
    sys.exit(main())
