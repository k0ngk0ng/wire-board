#!/usr/bin/env python3
"""Run source-bound pixel-only scenery terrain regressions; no gameplay policy."""

import hashlib
import json
import os
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
EVIDENCE = ROOT / "docs/research/nes-contra/evidence"


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    temporary = ROOT / ".local/contra-scenery-terrain/tmp"
    temporary.mkdir(parents=True, exist_ok=True)
    env = {
        **os.environ,
        "GOCACHE": str(ROOT / ".local/go-build"),
        "GOMODCACHE": str(ROOT / ".local/go-mod"),
        "GOTMPDIR": str(temporary),
        "TMPDIR": str(temporary),
        "GOTOOLCHAIN": "local",
    }
    files = sorted((ROOT / "games/contra/engine").glob("bot_vision*.go"))
    files += sorted((ROOT / "games/contra/engine/testdata").glob("bot-*.png"))
    files += [
        ROOT / "games/contra/engine/bot_test_resources_test.go",
        ROOT / "games/contra/engine/testdata/bot-visual-catalog.json.gz",
        Path(__file__).resolve(),
    ]
    before = {str(p.relative_to(ROOT)): sha(p) for p in files}
    command = [
        "go", "test", "-race", "./games/contra/engine", "-run",
        "^(TestBotSceneryTerrain|TestBotPlayerCovered|TestBotMaskedTerrain|"
        "TestBotEnergyFloor|TestBotVision|TestBuiltinBotVision|"
        "TestBotFutureSprite|TestBotSnowBackground)",
        "-count=1", "-json",
    ]
    log = EVIDENCE / "bot-scenery-terrain-race.jsonl"
    with log.open("w") as output:
        run = subprocess.run(command, cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT)
    changed = [str(p.relative_to(ROOT)) for p in files if sha(p) != before[str(p.relative_to(ROOT))]]
    rows = []
    non_json = []
    for line in log.read_text().splitlines():
        try:
            rows.append(json.loads(line))
        except json.JSONDecodeError:
            non_json.append(line)
    passed = [r["Test"] for r in rows if r.get("Action") == "pass" and r.get("Test")]
    failures = [r for r in rows if r.get("Action") == "fail"]
    result = {
        "passed": run.returncode == 0 and not changed and not failures and bool(passed),
        "command": command,
        "exitCode": run.returncode,
        "topLevelPassed": len([name for name in passed if "/" not in name]),
        "testAndSubtestPassed": len(passed),
        "failures": failures,
        "nonJSONOutput": non_json,
        "sourceChangedDuringCheck": changed,
        "sourceHashes": before,
        "logSHA256": sha(log),
        "redLogSHA256": sha(EVIDENCE / "bot-scenery-terrain-red.jsonl"),
        "scope": "Final RGBA and fixed artwork only. Accepted exact scenery196 opaque pixels join a separate terrain mask; player-only moving-support mask is unchanged. No policy, navigation, Session, World or OAM input.",
        "fixtures": {
            "119968": {"targetSceneryAnchor": [183, 176], "exactSceneryCount": 2},
            "129000": {"targetSceneryAnchor": [183, 176], "exactSceneryCount": 1},
            "targetSceneryRectangle": [181, 177, 4, 14],
            "targetSceneryOpaquePixels": 28,
            "recoveredPublicCoordinates": [[168, 180], [176, 180]],
            "negative": "Damaging one required seam pixel leaves both coordinates unknown.",
        },
        "limitation": "Pixel recognition and existing vision regressions only; does not certify navigation progress or a completed campaign.",
    }
    target = EVIDENCE / "bot-scenery-terrain-verification.json"
    target.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    temporary.rmdir()
    print(json.dumps({k: result[k] for k in ("passed", "topLevelPassed", "testAndSubtestPassed", "sourceChangedDuringCheck")}, ensure_ascii=False))
    raise SystemExit(0 if result["passed"] else 1)


if __name__ == "__main__":
    main()
