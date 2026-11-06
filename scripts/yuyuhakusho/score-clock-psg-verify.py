"""Repeat the frozen PSG real-owner checks with local-only Go state."""
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
    first = json.loads((ROOT / 'games/yuyuhakusho/verification/score-clock-psg/test-first.json').read_text())
    assert first['actual_missing_api_red'] and not first['implementation_exists']
    for path, digest in first['source_sha256'].items():
        assert sha(ROOT / path) == digest, ('Frozen pre-implementation input changed', path)
    package = ROOT / 'games/yuyuhakusho/audio/scoreclock'
    sources = [package / 'psg.go', package / 'psg_test.go', package / 'service.go', package / 'service_test.go']
    sources += list((package / 'testdata').glob('psg-*'))
    sources += list((ROOT / 'games/yuyuhakusho/audio/chips').glob('*.go'))
    sources += list((ROOT / 'games/yuyuhakusho/audio/chips/psg').glob('*.go'))
    sources += list((ROOT / 'games/yuyuhakusho/audio/dsp').glob('*.go'))
    hashes = {str(path.relative_to(ROOT)): sha(path) for path in sources}
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'), GOCACHE=str(ROOT / '.local/go-cache'),
               GOMODCACHE=str(ROOT / '.local/go-mod'), GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), TMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    for key in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR']:
        Path(env[key]).mkdir(parents=True, exist_ok=True)
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=5m', '-run', 'TestPSG']
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
    assert all(sha(ROOT / path) == digest for path, digest in hashes.items()), 'Sources changed during PSG verification'
    result = {'mode': args.mode, 'pass': completed.returncode == 0, 'exit_code': completed.returncode,
              'elapsed_seconds': time.monotonic() - started, 'command': command,
              'environment': {key: env[key] for key in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if key in env},
              'source_sha256': hashes, 'frozen_test_first_sha256': sha(ROOT / 'games/yuyuhakusho/verification/score-clock-psg/test-first.json'),
              'fixture_cases': 138, 'expected_writes': 192, 'log_sha256': sha(log)}
    (out / (args.mode + '-command.json')).write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps({key: result[key] for key in ['mode', 'pass', 'exit_code', 'elapsed_seconds', 'log_sha256']}, indent=2))
    if completed.returncode:
        print(log.read_text()[-10000:])
        raise SystemExit(completed.returncode)

if __name__ == '__main__':
    main()
