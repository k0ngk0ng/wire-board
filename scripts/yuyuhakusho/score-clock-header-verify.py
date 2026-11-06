"""Repeat frozen header native-race/Wasm/vet checks with local-only Go state."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--mode', choices=['native', 'wasm', 'vet'], required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    evidence = ROOT / 'games/yuyuhakusho/verification/score-clock-header'
    first = json.loads((evidence / 'test-first.json').read_text())
    assert first['test_first'] and not first['new_production_exists']
    for path, digest in first['files'].items():
        assert sha(ROOT / path) == digest, ('Frozen header input changed', path)
    package = ROOT / 'games/yuyuhakusho/audio/scoreclock'
    assert sha(package / 'header_test.go') == sha(evidence / 'header_test.go.txt')
    sources = [package / name for name in ['header.go', 'header_test.go', 'service.go', 'service_test.go', 'psg.go', 'psg_test.go', 'tick.go', 'tick_test.go']]
    sources += list((package / 'testdata').glob('header-*')) + [Path(__file__), evidence / 'header_test.go.txt']
    hashes = {str(path.relative_to(ROOT)): sha(path) for path in sources}
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'), GOCACHE=str(ROOT / '.local/go-cache'), GOMODCACHE=str(ROOT / '.local/go-mod'),
               GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), TMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    for key in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR']:
        Path(env[key]).mkdir(parents=True, exist_ok=True)
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=5m', '-run', '^TestHeader']
    if args.mode == 'native':
        command.append('-race')
    elif args.mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        executor = Path(goroot) / 'lib/wasm/go_js_wasm_exec'
        assert executor.is_file()
        command += ['-exec', str(executor)]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1')
    else:
        command = [go, 'vet']
    command.append('./games/yuyuhakusho/audio/scoreclock')
    started = time.monotonic()
    completed = subprocess.run(command, cwd=ROOT, env=env, capture_output=True, text=True)
    log = out / (args.mode + ('.txt' if args.mode == 'vet' else '.jsonl'))
    log.write_text(completed.stdout + completed.stderr)
    assert all(sha(ROOT / path) == digest for path, digest in hashes.items()), 'Header sources changed during verification'
    report = {'mode': args.mode, 'pass': completed.returncode == 0, 'exit_code': completed.returncode, 'elapsed_seconds': time.monotonic() - started,
              'command': command, 'environment': {key: env[key] for key in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if key in env},
              'source_sha256': hashes, 'frozen_test_first_sha256': sha(evidence / 'test-first.json'), 'fixture_cases': 170, 'routine_stops': 330, 'log_sha256': sha(log)}
    (out / (args.mode + '-command.json')).write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({key: report[key] for key in ['mode', 'pass', 'exit_code', 'elapsed_seconds', 'log_sha256']}, indent=2))
    if completed.returncode:
        print(log.read_text()[-12000:])
        raise SystemExit(completed.returncode)

if __name__ == '__main__':
    main()
