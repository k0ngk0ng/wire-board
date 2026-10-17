"""Opt-in cold complete-match diagnostics; no platform options or default matrix."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
TEST = "TestRuntimeTeamRosterDiagnostic"
PRIORITY = [dict(court=1, teams=[0, 1], characters=[2, 3, 0, 1])] + [
    dict(court=1, teams=[0, team], characters=[0, 1, 0, 1]) for team in range(2, 8)
]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def identity():
    paths = sorted((ROOT / "games/basketball/engine").glob("*.go"))
    paths += [ROOT / "go.mod", ROOT / "go.sum", Path(__file__).resolve()]
    files = {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in paths if path.exists()}
    return dict(sha256=sha(json.dumps(files, sort_keys=True).encode()), files=files)


def selection_id(selection):
    return "court{}-teams{}-characters{}".format(selection["court"],
        "-".join(map(str, selection["teams"])), "-".join(map(str, selection["characters"])))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--priority", action="store_true", help="run exactly seven Court1 configurations")
    group.add_argument("--teams", type=int, nargs=2, metavar=("HOME", "AWAY"))
    parser.add_argument("--characters", type=int, nargs=4)
    parser.add_argument("--court", type=int, choices=(0, 1), default=1)
    parser.add_argument("--wasm", action="store_true")
    parser.add_argument("--list", action="store_true", help="print selections without running")
    parser.add_argument("--output", default=".local/basketball/team-roster-diagnostics")
    args = parser.parse_args()
    if args.characters is not None and args.teams is None:
        parser.error("--characters requires --teams")
    if args.teams is not None:
        if args.characters is None:
            parser.error("--teams requires four explicit --characters")
        if any(team not in range(8) for team in args.teams) or len(set(args.teams)) != 2:
            parser.error("select two distinct teams from 0..7")
        if any(char not in range(4) for char in args.characters) or any(
                args.characters[i] == args.characters[i + 1] for i in (0, 2)):
            parser.error("characters must be 0..3 and distinct within each team")
        selections = [dict(court=args.court, teams=args.teams, characters=args.characters)]
    else:
        if args.court != 1:
            parser.error("the priority batch is fixed to Court1")
        selections = PRIORITY
    if args.list or not (args.priority or args.teams):
        print(json.dumps(dict(executed=False, selections=selections), indent=2))
        return 0
    output = (ROOT / args.output).resolve()
    if not output.is_relative_to(ROOT) or output == ROOT:
        parser.error("output must be a directory inside the workspace")
    flavor = "wasm" if args.wasm else "native"
    output = output / flavor
    if output.exists() and any(output.iterdir()):
        parser.error("output already contains evidence; choose a fresh --output")
    output.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    for key, folder in (("GOCACHE", "go-build"), ("GOMODCACHE", "go-mod"), ("GOTMPDIR", "tmp")):
        env[key] = str(ROOT / ".local" / folder)
        Path(env[key]).mkdir(parents=True, exist_ok=True)
    env.update(TMPDIR=env["GOTMPDIR"], GOTOOLCHAIN="local", BASKETBALL_RUNTIME_DIAGNOSTIC="1")
    command = ["go", "test", "-json", "./games/basketball/engine", "-run", "^" + TEST + "$", "-count=1", "-timeout=15m"]
    if args.wasm:
        goroot = subprocess.check_output(["go", "env", "GOROOT"], cwd=ROOT, env=env, text=True).strip()
        command += ["-exec", str(Path(goroot) / "lib/wasm/go_js_wasm_exec")]
        env.update(GOOS="js", GOARCH="wasm")
    else:
        # A caller's cross-build environment must not mislabel native results.
        env.pop("GOOS", None)
        env.pop("GOARCH", None)
    report = dict(scope="cold configuration; both natural periods, original AI, periodic JSON restore; not ROM/menu/campaign or all-character coverage",
                  seed=0x12345678, maximumVideoFrames=48000, repeats=2,
                  controllerAssignments=[-1] * 4, flavor=flavor, cases=[])
    (output / "plan.json").write_text(json.dumps(dict(command=command, selections=selections), indent=2) + "\n")
    for selection in selections:
        name = selection_id(selection)
        directory = output / name
        directory.mkdir()
        env["BASKETBALL_RUNTIME_SELECTION"] = json.dumps(selection, separators=(",", ":"))
        before_source = identity()
        (directory / "source-before.json").write_text(json.dumps(before_source, indent=2) + "\n")
        print("Running " + name, flush=True)
        with (directory / "tests.jsonl").open("wb") as log:
            result = subprocess.run(command, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
        after_source = identity()
        (directory / "source-after.json").write_text(json.dumps(after_source, indent=2) + "\n")
        raw = (directory / "tests.jsonl").read_bytes()
        events = []
        for line in raw.decode(errors="replace").splitlines():
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                pass
        transcript = "".join(event.get("Output", "") for event in events)
        (directory / "transcript.log").write_text(transcript)
        stable = before_source == after_source
        passed = result.returncode == 0 and any(event.get("Action") == "pass" and event.get("Test") == TEST for event in events)
        case = dict(id=name, selection=selection, command=command,
                    environment={key: env[key] for key in ("BASKETBALL_RUNTIME_DIAGNOSTIC", "BASKETBALL_RUNTIME_SELECTION")},
                    exitCode=result.returncode, testPassed=passed, sourceStable=stable, accepted=passed and stable,
                    sourceSHA256=before_source["sha256"], logSHA256=sha(raw),
                    log=str((directory / "tests.jsonl").relative_to(ROOT)))
        failure = re.search(r"runtime cold seed 12345678 repeat (\d+) video (\d+)/(\d+) logic (\d+): ([^\n]+)", transcript)
        if failure:
            case.update(repeat=int(failure[1]), video=int(failure[2]), logic=int(failure[4]), failure=failure[5])
        before = re.search(r"^\s*before=(.+)$", transcript, re.MULTILINE)
        if before:
            state = json.loads(before[1])
            state_bytes = (json.dumps(state, indent=2) + "\n").encode()
            (directory / "before.json").write_bytes(state_bytes)
            case.update(before=str((directory / "before.json").relative_to(ROOT)), beforeSHA256=sha(state_bytes))
        case["completedRuns"] = [dict(repeat=int(match[1]), videos=int(match[2]), logic=int(match[3]), restores=int(match[4]), trajectorySHA256=match[5])
            for match in re.finditer(r"runtime cold repeat=(\d+) videos=(\d+) logic=(\d+) restores=(\d+).*trajectory_sha256=([0-9a-f]{64})", transcript)]
        (directory / "result.json").write_text(json.dumps(case, indent=2) + "\n")
        report["cases"].append(case)
        report["accepted"] = len(report["cases"]) == len(selections) and all(item["accepted"] for item in report["cases"])
        (output / "report.json").write_text(json.dumps(report, indent=2) + "\n")
        print(json.dumps(case), flush=True)
        if not stable:
            print("Stopping: source changed during this configuration; result is not accepted.", file=sys.stderr)
            break
    return 0 if report["accepted"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
