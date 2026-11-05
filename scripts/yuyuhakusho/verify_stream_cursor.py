"""Source-bound cursor transport acceptance; never runs the 32 AI matrix."""

from datetime import datetime, timezone
from pathlib import Path
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
VERIFY = ROOT / "games/yuyuhakusho/verification"
PACKAGES = ["./internal/gameapi", "./internal/server", "./games/yuyuhakusho/server", "./games/yuyuhakusho/platformtest"]
SELECTION = "^Test(RealtimeCursor|RealtimeStream(BootstrapAndScopedPush|InputAcknowledgedExactlyOnce|RejectsSpectatorAndStaleRoom)$|Stream(Delta|Cursor)|CursorHTTPWSReplaySizesAndReconnect$|Module(WireAndRecoveryBoundaries|SyncSnapshotE2E)$|RealSlowSpectatorDoesNotBlockFastPlayerOrClock$)"


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    record = json.loads((VERIFY / "platform-stream-cursor-test-first.json").read_text())
    expected = dict(record["frozen_tests"])
    prior = json.loads((VERIFY / "platform-module-final-validation.json").read_text())
    expected.update(prior["frozen_preimplementation_files_sha256"])
    for name, digest in expected.items():
        if sha(ROOT / name) != digest:
            raise RuntimeError(f"Frozen test source changed: {name}")
    return expected


def source_files(go, env, packages):
    command = [go, "list", "-deps", "-test", "-json", *packages]
    raw = subprocess.check_output(command, cwd=ROOT, env=env, text=True)
    decoder = json.JSONDecoder()
    paths = {ROOT / "go.mod", ROOT / "go.sum", Path(__file__).resolve()}
    while raw.strip():
        raw = raw.lstrip()
        package, end = decoder.raw_decode(raw)
        raw = raw[end:]
        directory = Path(package.get("Dir", "/"))
        if not directory.is_relative_to(ROOT) or directory.is_relative_to(ROOT / ".local"):
            continue
        for field in ("GoFiles", "CgoFiles", "TestGoFiles", "XTestGoFiles", "CFiles", "HFiles", "SFiles", "EmbedFiles", "TestEmbedFiles", "XTestEmbedFiles"):
            for name in package.get(field, []):
                path = (directory / name).resolve()
                if path.is_file() and path.is_relative_to(ROOT) and not path.is_relative_to(ROOT / ".local"):
                    paths.add(path)
    return command, {str(path.relative_to(ROOT)): sha(path) for path in sorted(paths)}


def run(suite, revision):
    frozen_before = frozen()
    go, node = shutil.which("go"), shutil.which("node")
    assert go and node
    temporary = ROOT / ".local/md-yuyuhakusho/tmp"
    temporary.mkdir(parents=True, exist_ok=True)
    overrides = dict(PATH=str(Path(go).parent) + ":" + str(Path(node).parent) + ":/usr/bin:/bin", GOCACHE=str(ROOT / ".local/go-cache"), GOMODCACHE=str(ROOT / ".local/go-mod"), GOTMPDIR=str(temporary), TMPDIR=str(temporary), TEST_TELEMETRY_DIR=str(ROOT / ".local/go-telemetry"), GOENV="off", GOTOOLCHAIN="local", YUYU_CURSOR_TEST_LABEL=suite)
    env = {key: os.environ[key] for key in ("HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ") if key in os.environ}
    env.update(overrides)
    packages = PACKAGES
    command = [go, "test", "-json", "-count=1", "-timeout=5m", "-run", SELECTION]
    if suite == "race":
        command[command.index(SELECTION)] = SELECTION.replace("CursorHTTPWSReplaySizesAndReconnect$", "CursorHTTPWSFunctionalRace$")
        command.append("-race")
    elif suite == "wasm":
        overrides.update(GOOS="js", GOARCH="wasm")
        env.update(overrides)
        packages = ["./games/yuyuhakusho/server"]
        goroot = subprocess.check_output([go, "env", "GOROOT"], cwd=ROOT, env=env, text=True).strip()
        command.extend(["-exec", str(Path(goroot) / "lib/wasm/go_js_wasm_exec")])
    elif suite == "vet":
        command = [go, "vet"]
    command.extend(packages)
    inventory_command, initial = source_files(go, env, packages)
    extension = "txt" if suite == "vet" else "jsonl"
    log = VERIFY / f"platform-stream-cursor-{suite}-{revision}.{extension}"
    record = VERIFY / f"platform-stream-cursor-{suite}-{revision}-command.json"
    metadata = dict(suite=suite, started_at_utc=datetime.now(timezone.utc).isoformat(), command=command, environment_overrides=overrides, source_inventory_command=inventory_command, source_sha256=initial, frozen_test_sha256=frozen_before, log=str(log.relative_to(ROOT)), go_version=subprocess.check_output([go, "version"], cwd=ROOT, env=env, text=True).strip(), node_version=subprocess.check_output([node, "--version"], cwd=ROOT, env=env, text=True).strip(), complete=False, old_263_manifest_sha256=sha(VERIFY / "platform-module-final-validation.json"))
    record.write_text(json.dumps(metadata, indent=2) + "\n")
    started = time.monotonic()
    with log.open("w") as output:
        result = subprocess.run(command, cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT)
    _, after = source_files(go, env, packages)
    metadata.update(complete=True, ended_at_utc=datetime.now(timezone.utc).isoformat(), exit_code=result.returncode, wall_seconds=round(time.monotonic() - started, 3), sources_unchanged=initial == after, log_sha256=sha(log), frozen_tests_unchanged=frozen() == frozen_before)
    if suite != "vet":
        events, unparsed = [], []
        for line in log.read_text().splitlines():
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                unparsed.append(line)
        metadata.update(package_results=[row for row in events if row.get("Action") in ("pass", "fail", "skip") and "Test" not in row], top_tests_passed=sum(row.get("Action") == "pass" and "Test" in row and "/" not in row["Test"] for row in events), subtests_passed=sum(row.get("Action") == "pass" and "/" in row.get("Test", "") for row in events), failed_tests=[row["Test"] for row in events if row.get("Action") == "fail" and "Test" in row], unparsed_output_lines=len(unparsed))
    if suite in ("native", "race") and result.returncode == 0:
        metadata["network_evidence"] = {}
        target = VERIFY / "platform-stream-cursor"
        target.mkdir(exist_ok=True)
        for name in ("cursor-slow-connection", "cursor-detached-scope"):
            source = ROOT / ".local/realtime-efficiency" / (name + ".json")
            destination = target / (suite + "-" + name + ".json")
            shutil.copyfile(source, destination)
            metadata["network_evidence"][str(destination.relative_to(ROOT))] = sha(destination)
        actual = target / ("actual-http-ws-" + suite + ".json")
        metadata["network_evidence"][str(actual.relative_to(ROOT))] = sha(actual)
    metadata["pass"] = result.returncode == 0 and metadata["sources_unchanged"] and metadata["frozen_tests_unchanged"]
    record.write_text(json.dumps(metadata, indent=2) + "\n")
    print(json.dumps({key: value for key, value in metadata.items() if key not in ("source_sha256", "frozen_test_sha256", "environment_overrides")}, indent=2))
    return 0 if metadata["pass"] else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("suite", choices=("native", "race", "wasm", "vet"))
    parser.add_argument("--revision", choices=("receive-final",), default="receive-final")
    args = parser.parse_args()
    raise SystemExit(run(args.suite, args.revision))
