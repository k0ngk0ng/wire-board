#!/usr/bin/env python3
"""Reject direct dependencies between game implementations."""
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
failures = []
games = [p for p in (root / "games").iterdir() if p.is_dir() and not p.name.startswith(("_", "."))]
for game in games:
    for area in ("server", "web"):
        directory = game / area
        for source in directory.rglob("*"):
            if source.suffix not in (".go", ".ts", ".tsx") or source.name.endswith("_test.go"):
                continue
            text = source.read_text()
            if source.suffix == ".go":
                imports = re.findall(r'"github.com/k0ngk0ng/wire-board/([^"\n]+)"', text)
                for target in imports:
                    if target.startswith("games/") and target.split("/")[1] != game.name:
                        failures.append(f"{source.relative_to(root)} imports {target}")
                    if target.startswith(("internal/server", "internal/game/")) or target == "internal/game":
                        failures.append(f"{source.relative_to(root)} imports legacy/platform implementation {target}")
            else:
                for target in re.findall(r'(?:from\s*|import\s*|import\(\s*)["\']([^"\']+)', text):
                    if target.startswith("."):
                        resolved = (source.parent / target).resolve()
                        if resolved.is_relative_to(root / "games") and not resolved.is_relative_to(game):
                            failures.append(f"{source.relative_to(root)} imports another game: {target}")
                        if resolved.is_relative_to(root / "web/src/legacy"):
                            failures.append(f"{source.relative_to(root)} imports legacy UI: {target}")
if failures:
    sys.exit("\n".join(failures))
print(f"PASS: {len(games)} games have no direct game-to-game or legacy implementation imports")
