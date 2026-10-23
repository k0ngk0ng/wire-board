#!/usr/bin/env python3
"""Repeat the historical-JAR probes without installing or launching the game."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--jar", type=Path, default=Path(".local/conquest-research/aoc-desktop.jar"))
    parser.add_argument("--jdk-bin", type=Path, help="Directory containing java and javac")
    args = parser.parse_args()
    root = Path.cwd().resolve()
    here = Path(__file__).resolve().parent
    output = root / ".local/conquest-research/probe"
    output.mkdir(parents=True, exist_ok=True)
    expected = json.loads((here / "legacy-probe-results.json").read_text())
    jar = args.jar.resolve()
    assert hashlib.sha256(jar.read_bytes()).hexdigest() == expected["referenceJarSHA256"], "Reference JAR SHA-256 mismatch"
    source = here / "LegacyRuleProbe.java"
    assert hashlib.sha256(source.read_bytes()).hexdigest() == expected["probeSHA256"], "Probe source changed; review and regenerate reference observations"
    java = str(args.jdk_bin / "java") if args.jdk_bin else shutil.which("java")
    javac = str(args.jdk_bin / "javac") if args.jdk_bin else shutil.which("javac")
    if not java or not javac:
        raise SystemExit("An existing JDK is required. Pass --jdk-bin; this script does not install one.")
    env = dict(os.environ, TMPDIR=str(output))
    subprocess.run([javac, "-J-XX:-UsePerfData", "-J-Djava.io.tmpdir=" + str(output),
                    "-cp", str(jar), "-d", str(output), str(source)], check=True, env=env)
    completed = subprocess.run([java, "-XX:-UsePerfData", "-Djava.io.tmpdir=" + str(output),
                                "-cp", str(output) + os.pathsep + str(jar), "LegacyRuleProbe"],
                               text=True, capture_output=True, check=True, env=env)
    actual = [json.loads(line) for line in completed.stdout.splitlines()]
    report = {"passed": actual == expected["results"], "observations": len(actual),
              "referenceJarSHA256": expected["referenceJarSHA256"],
              "probeSHA256": expected["probeSHA256"], "actual": actual}
    destination = output.parent / "probe-verification.json"
    destination.write_text(json.dumps(report, indent=2) + "\n")
    if not report["passed"]:
        raise SystemExit("Historical probe observations changed: " + str(destination))
    print(json.dumps({"passed": True, "observations": len(actual), "report": str(destination)}))


if __name__ == "__main__":
    main()
