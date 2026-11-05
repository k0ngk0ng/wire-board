"""Repeatable native/Wasm/race checks for the YuYu server and shared tick."""

from datetime import datetime, timezone
from pathlib import Path
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
import time


ROOT = Path(__file__).resolve().parents[2]
VERIFY = ROOT / "games/yuyuhakusho/verification"
PACKAGES = ["./games/yuyuhakusho/server", "./games/yuyuhakusho/sessiontick"]


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen_tests():
    manifest = json.loads((VERIFY / "platform-module-test-first.json").read_text())
    expected = dict(manifest["files"])
    for name in ("platform-module-resource-test-first.json", "platform-module-transport-test-first.json", "platform-module-controller-test-first.json", "platform-module-numeric-json-test-first.json"):
        path = VERIFY / name
        if path.exists():
            extension = json.loads(path.read_text())
            expected.update(extension.get("test_sha256", extension.get("files", {})))
    for name, digest in expected.items():
        if sha(ROOT / name) != digest:
            raise RuntimeError(f"Frozen preimplementation test changed: {name}")
    return expected


def sources():
    paths = []
    for package in ("server", "sessiontick", "engine", "renderstate", "resourceid", "syncwire", "data"):
        paths.extend((ROOT / "games/yuyuhakusho" / package).glob("*.go"))
    paths.extend((ROOT / "internal/gameapi").glob("*.go"))
    for name in ("chu-world-rules.json.gz", "renderstate-metadata.json.gz", "runtime-resource-descriptor.json"):
        paths.append(ROOT / "games/yuyuhakusho/data" / name)
    paths.extend([ROOT / "go.mod", ROOT / "go.sum", Path(__file__).resolve()])
    return {str(path.relative_to(ROOT)): sha(path) for path in sorted(set(paths))}


def run(suite, revision):
    frozen = frozen_tests()
    initial_sources = sources()
    go = shutil.which("go")
    node = shutil.which("node")
    if not go or not node:
        raise RuntimeError("Go and Node must be available for native/Wasm verification")
    temporary = ROOT / ".local/md-yuyuhakusho/tmp"
    temporary.mkdir(parents=True, exist_ok=True)
    env = {key: os.environ[key] for key in ("HOME", "LANG", "LC_ALL", "LC_CTYPE", "TZ") if key in os.environ}
    overrides = {
        "PATH": str(Path(go).parent) + ":" + str(Path(node).parent) + ":/usr/bin:/bin",
        "GOCACHE": str(ROOT / ".local/go-cache"),
        "GOMODCACHE": str(ROOT / ".local/go-mod"),
        "GOTMPDIR": str(temporary),
        "TMPDIR": str(temporary),
        "TEST_TELEMETRY_DIR": str(ROOT / ".local/go-telemetry"),
        "GOENV": "off",
        "GOTOOLCHAIN": "local",
    }
    env.update(overrides)
    command = [go, "test", "-json", "-count=1", "-timeout=30m"]
    if suite == "wasm":
        overrides.update(GOOS="js", GOARCH="wasm")
        env.update(overrides)
        goroot = subprocess.check_output([go, "env", "GOROOT"], env=env, cwd=ROOT, text=True).strip()
        command.extend(["-exec", str(Path(goroot) / "lib/wasm/go_js_wasm_exec")])
    elif suite == "race":
        command.extend(["-race", "-run", "^Test(SharedTick|Waiting|Module(Initial|Invalid|Human|Wire|Draw|Saved|Transport|Phase|Controller))"])
    elif suite == "vet":
        command = [go, "vet"]
    command.extend(PACKAGES)
    extension = "txt" if suite == "vet" else "jsonl"
    log = VERIFY / f"platform-module-{suite}-{revision}.{extension}"
    record = VERIFY / f"platform-module-{suite}-{revision}-command.json"
    metadata = {
        "suite": suite,
        "revision": revision,
        "started_at_utc": datetime.now(timezone.utc).isoformat(),
        "command": command,
        "environment_overrides": overrides,
        "go_version": subprocess.check_output([go, "version"], env=env, cwd=ROOT, text=True).strip(),
        "node_version": subprocess.check_output([node, "--version"], env=env, cwd=ROOT, text=True).strip(),
        "frozen_test_sha256": frozen,
        "source_sha256": initial_sources,
        "log": str(log.relative_to(ROOT)),
        "complete": False,
    }
    record.write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n")
    started = time.monotonic()
    with log.open("w") as output:
        result = subprocess.run(command, cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT)
    metadata.update(
        complete=True,
        ended_at_utc=datetime.now(timezone.utc).isoformat(),
        wall_seconds=round(time.monotonic() - started, 3),
        exit_code=result.returncode,
        log_sha256=sha(log),
        sources_unchanged=sources() == initial_sources,
    )
    frozen_tests()
    if suite != "vet":
        events = []
        unparsed = []
        for line in log.read_text().splitlines():
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                unparsed.append(line)
        metadata["package_results"] = [row for row in events if row.get("Action") in ("pass", "fail", "skip") and "Test" not in row]
        metadata["top_level_tests_passed"] = sum(row.get("Action") == "pass" and "Test" in row and "/" not in row["Test"] for row in events)
        metadata["subtests_passed"] = sum(row.get("Action") == "pass" and "/" in row.get("Test", "") for row in events)
        metadata["failed_tests"] = [row["Test"] for row in events if row.get("Action") == "fail" and "Test" in row]
        metadata["unparsed_output_lines"] = len(unparsed)
    metadata["pass"] = result.returncode == 0 and metadata["sources_unchanged"]
    record.write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({key: value for key, value in metadata.items() if key not in ("source_sha256", "frozen_test_sha256", "environment_overrides")}, ensure_ascii=False, indent=2))
    return 0 if metadata["pass"] else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("suite", choices=("native", "wasm", "race", "vet"))
    parser.add_argument("--revision", choices=("final", "review-final", "wire-final"), default="wire-final")
    args = parser.parse_args()
    sys.exit(run(args.suite, args.revision))
