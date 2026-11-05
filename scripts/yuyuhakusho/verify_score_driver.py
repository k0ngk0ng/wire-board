"""Run the frozen continuous Go driver checks with bound sources and fixtures."""
from pathlib import Path
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main(out, mode):
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    package = ROOT / 'games/yuyuhakusho/audio/score'
    index_path = package / 'testdata/original-index.json'
    index = json.loads(index_path.read_text())
    for case in index['cases']:
        path = (index_path.parent / case['path']).resolve()
        assert path.is_relative_to(index_path.parent) and sha(path) == case['sha256']
    sources = list(package.glob('*.go')) + list((package / 'param').glob('*.go'))
    sources += [ROOT / 'games/yuyuhakusho/data/audio-score.json.gz', index_path]
    source_hashes = {str(path.relative_to(ROOT)): sha(path) for path in sources}
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'),
               GOCACHE=str(ROOT / '.local/go-cache'),
               GOMODCACHE=str(ROOT / '.local/go-mod'),
               GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'))
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=30m']
    if mode == 'native':
        command.append('-race')
    elif mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        executor = Path(goroot) / 'lib/wasm/go_js_wasm_exec'
        assert executor.is_file()
        command += ['-exec', str(executor)]
        env.update(GOOS='js', GOARCH='wasm')
    else:
        command = [go, 'vet']
    command.append('./games/yuyuhakusho/audio/score')
    started = time.monotonic()
    completed = subprocess.run(command, cwd=ROOT, env=env, capture_output=True, text=True)
    log = out / (mode + ('.txt' if mode == 'vet' else '.jsonl'))
    log.write_text(completed.stdout + completed.stderr)
    assert all(sha(ROOT / path) == digest for path, digest in source_hashes.items()), 'sources changed during verification'
    result = {'mode': mode, 'pass': completed.returncode == 0, 'exit_code': completed.returncode,
              'elapsed_seconds': time.monotonic() - started, 'command': command,
              'environment': {key: env[key] for key in ('TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOOS', 'GOARCH') if key in env},
              'source_sha256': source_hashes, 'fixture_cases': len(index['cases']),
              'fixture_frames': sum(case['steps'] for case in index['cases']),
              'log_sha256': sha(log)}
    (out / (mode + '-command.json')).write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))
    if completed.returncode:
        print((completed.stdout + completed.stderr)[-12000:])
        raise SystemExit(completed.returncode)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--mode', choices=('native', 'wasm', 'vet'), required=True)
    args = parser.parse_args()
    main(args.out.resolve(), args.mode)
