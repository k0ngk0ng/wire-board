"""Verify default build-tag selection; preserve prior cursor revision records."""

from datetime import datetime, timezone
from pathlib import Path
import argparse
import hashlib
import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
import time

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "games/yuyuhakusho/verification/platform-stream-cursor-ci"
PACKAGE = "./games/yuyuhakusho/platformtest"
PERFORMANCE = "TestCursorHTTPWSReplaySizesAndReconnect"
FUNCTIONAL = "TestCursorHTTPWSFunctionalRace"


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def verify_organization():
    before = json.loads((OUT / "before-organization.json").read_text())
    original = (ROOT / before["original_test_archive"]).read_text()
    organization = json.loads((OUT / "organization.json").read_text())
    block = organization["wrapper_exact_source"]
    assert hashlib.sha256(original.encode()).hexdigest() == before["before_shared_test_sha256"]
    assert original.count(block) == 1
    shared = ROOT / "games/yuyuhakusho/platformtest/stream_cursor_test.go"
    native = ROOT / "games/yuyuhakusho/platformtest/native_performance_test.go"
    assert shared.read_text() == original.replace(block, ""), "Shared driver/functionality changed"
    assert native.read_text() == '//go:build !race\n\npackage platformtest\n\nimport "testing"\n\n' + block.rstrip() + "\n"
    for name, digest in before["production_sha256"].items():
        assert sha(ROOT / name) == digest, f"Production changed: {name}"
    for name, key in (("platform-stream-cursor-final-validation.json", "prior_final_manifest_sha256"), ("platform-module-final-validation.json", "prior_263_manifest_sha256")):
        assert sha(ROOT / "games/yuyuhakusho/verification" / name) == before[key]
    return {str(path.relative_to(ROOT)): sha(path) for path in (shared, native, Path(__file__).resolve())}


def run(suite):
    organization = verify_organization()
    go, node = shutil.which("go"), shutil.which("node")
    assert go and node
    temporary = ROOT / ".local/md-yuyuhakusho/tmp"
    temporary.mkdir(parents=True, exist_ok=True)
    overrides = dict(PATH=str(Path(go).parent) + ":" + str(Path(node).parent) + ":/usr/bin:/bin", GOCACHE=str(ROOT / ".local/go-cache"), GOMODCACHE=str(ROOT / ".local/go-mod"), GOTMPDIR=str(temporary), TMPDIR=str(temporary), TEST_TELEMETRY_DIR=str(ROOT / ".local/go-telemetry"), GOENV="off", GOTOOLCHAIN="local", PYTHONDONTWRITEBYTECODE="1")
    env = {key: os.environ[key] for key in ("HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ") if key in os.environ}
    env.update(overrides)
    specification = importlib.util.spec_from_file_location("cursor_inventory", ROOT / "scripts/yuyuhakusho/verify_stream_cursor.py")
    inventory = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(inventory)
    packages = [PACKAGE] if suite != "host-race" else ["./internal/server"]
    _, initial = inventory.source_files(go, env, packages)
    initial.update(organization)
    started = time.monotonic()
    commands = []
    record = dict(suite=suite, revision="cursor-ci-buildtag", started_at_utc=datetime.now(timezone.utc).isoformat(), source_sha256=initial, environment_overrides=overrides, complete=False)
    path = OUT / (suite + "-command.json")
    path.write_text(json.dumps(record, indent=2) + "\n")
    if suite == "discovery":
        names = {}
        for mode in ("native", "race"):
            command = [go, "test", "-json", "-list", "."]
            if mode == "race":
                command.append("-race")
            command.append(PACKAGE)
            commands.append(command)
            result = subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True)
            (OUT / (mode + "-discovery.jsonl")).write_text(result.stdout + result.stderr)
            assert result.returncode == 0
            names[mode] = sorted({row["Output"].strip() for line in result.stdout.splitlines() if (row := json.loads(line)).get("Action") == "output" and re.fullmatch(r"Test\w+", row.get("Output", "").strip())})
        assert PERFORMANCE in names["native"] and PERFORMANCE not in names["race"]
        assert FUNCTIONAL in names["native"] and FUNCTIONAL in names["race"]
        assert set(names["native"]) - set(names["race"]) == {PERFORMANCE}
        assert not set(names["race"]) - set(names["native"])
        record.update(names=names, native_only=[PERFORMANCE], race_only=[], exit_code=0)
    else:
        command = [go, "test", "-json", "-count=1", "-timeout=5m"]
        if suite == "native-performance":
            command.extend(["-run", "^" + PERFORMANCE + "$"])
        elif suite == "default-race":
            command.append("-race")
        else:
            command.extend(["-race", "-run", "^Test(RealtimeCursor|RealtimeStream(BootstrapAndScopedPush|InputAcknowledgedExactlyOnce|RejectsSpectatorAndStaleRoom)$|ModuleSyncSnapshotE2E$)"])
        command.extend(packages)
        commands.append(command)
        log = OUT / (suite + ".jsonl")
        with log.open("w") as output:
            result = subprocess.run(command, cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT)
        rows = [json.loads(line) for line in log.read_text().splitlines()]
        executed = [row["Test"] for row in rows if row.get("Action") == "run" and "/" not in row.get("Test", "")]
        if suite == "default-race":
            assert PERFORMANCE not in executed and FUNCTIONAL in executed
        if suite == "native-performance":
            assert executed == [PERFORMANCE]
        record.update(exit_code=result.returncode, executed_tests=executed, failed_tests=[row["Test"] for row in rows if row.get("Action") == "fail" and "Test" in row], log=str(log.relative_to(ROOT)), log_sha256=sha(log), package_results=[row for row in rows if row.get("Action") in ("pass", "fail") and "Test" not in row])
        if suite in ("native-performance", "default-race") and result.returncode == 0:
            source = ROOT / "games/yuyuhakusho/verification/platform-stream-cursor/actual-http-ws-manual.json"
            destination = OUT / (suite + "-transport.json")
            shutil.copyfile(source, destination)
            record["transport_evidence"] = {str(destination.relative_to(ROOT)): sha(destination)}
    _, after = inventory.source_files(go, env, packages)
    after.update(verify_organization())
    record.update(commands=commands, complete=True, sources_unchanged=after == initial, wall_seconds=round(time.monotonic() - started, 3), ended_at_utc=datetime.now(timezone.utc).isoformat())
    record["pass"] = record["exit_code"] == 0 and record["sources_unchanged"]
    path.write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps({key: value for key, value in record.items() if key not in ("source_sha256", "environment_overrides")}, indent=2))
    return 0 if record["pass"] else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("suite", choices=("discovery", "native-performance", "default-race", "host-race"))
    raise SystemExit(run(parser.parse_args().suite))
