"""Bound observed/legacy/native regression, without overwriting old proofs."""
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
    assert not out.is_relative_to(ROOT / 'games/yuyuhakusho/verification/audio-score-driver')
    out.mkdir(parents=True, exist_ok=True)
    package = ROOT / 'games/yuyuhakusho/audio/score'
    prior = json.loads((ROOT / 'games/yuyuhakusho/verification/audio-score-driver/frozen-sources.json').read_text())['bindings']
    retained = ['games/yuyuhakusho/audio/score/state.go',
                'games/yuyuhakusho/audio/score/driver_test.go',
                'games/yuyuhakusho/audio/score/restore_contract_test.go',
                'games/yuyuhakusho/data/audio-score.json.gz',
                'games/yuyuhakusho/audio/score/testdata/original-index.json']
    assert all(sha(ROOT / path) == prior[path] for path in retained)
    index_path = package / 'testdata/original-index.json'
    index = json.loads(index_path.read_text())
    assert len(index['cases']) == 42
    for case in index['cases']:
        path = (index_path.parent / case['path']).resolve()
        assert path.is_relative_to(index_path.parent) and sha(path) == case['sha256']
    sources = list(package.glob('*.go')) + list((package / 'param').glob('*.go'))
    sources += [ROOT / 'games/yuyuhakusho/data/audio-score.json.gz',
                ROOT / 'games/yuyuhakusho/data/audio-score-parser-work.json.gz',
                index_path, Path(__file__).resolve()]
    bindings = {str(path.relative_to(ROOT)): sha(path) for path in sources}
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'),
               GOCACHE=str(ROOT / '.local/go-cache'), GOMODCACHE=str(ROOT / '.local/go-mod'),
               GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'))
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=45m']
    if mode == 'native':
        command.append('-race')
    elif mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command += ['-exec', str(Path(goroot) / 'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm')
    else:
        command = [go, 'vet']
    command.append('./games/yuyuhakusho/audio/score')
    start = time.monotonic()
    log = out / (mode + ('.txt' if mode == 'vet' else '.jsonl'))
    # Keep live output on disk so long original trajectories remain inspectable.
    with log.open('w') as stream:
        process = subprocess.Popen(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT, text=True)
        code = process.wait()
    assert all(sha(ROOT / path) == digest for path, digest in bindings.items()), 'sources changed during verification'
    result = {'mode': mode, 'pass': code == 0, 'exit_code': code,
              'elapsed_seconds': time.monotonic() - start, 'command': command,
              'environment': {key: env[key] for key in ('TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOOS', 'GOARCH') if key in env},
              'source_sha256': bindings, 'old_42_fixtures_and_state_tests_retained': True,
              'log_sha256': sha(log)}
    (out / (mode + '-command.json')).write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))
    if code:
        print(log.read_text()[-12000:])
        raise SystemExit(code)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--mode', choices=('native', 'wasm', 'vet'), required=True)
    args = parser.parse_args()
    main(args.out.resolve(), args.mode)
