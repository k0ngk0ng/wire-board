"""Verify zero-tick control actions and staged score without replacing proofs."""
from pathlib import Path
import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def state_layout(text):
    return re.sub(r'\s+', ' ', text.split('type State struct {', 1)[1].split('\n}', 1)[0]).strip()


def main(out, mode):
    assert out.is_relative_to(ROOT)
    for old in ('audio-score-driver', 'audio-score-observed'):
        assert not out.is_relative_to(ROOT / 'games/yuyuhakusho/verification' / old)
    out.mkdir(parents=True, exist_ok=True)
    package = ROOT / 'games/yuyuhakusho/audio/score'
    previous = json.loads((ROOT / 'games/yuyuhakusho/verification/audio-score-observed/frozen-sources.json').read_text())['bindings']
    for path, digest in previous.items():
        if path.endswith('_test.go') or path.endswith('audio-score.json.gz') or path.endswith('audio-score-parser-work.json.gz') or path.endswith('original-index.json'):
            assert sha(ROOT / path) == digest, path
    layout_path = ROOT / 'games/yuyuhakusho/verification/audio-score-control/state-layout.json'
    layout = json.loads(layout_path.read_text())
    assert state_layout((package / 'state.go').read_text()) == layout['canonical_layout']
    index_path = package / 'testdata/original-index.json'
    original = json.loads(index_path.read_text())
    assert len(original['cases']) == 42
    for case in original['cases']:
        path = (index_path.parent / case['path']).resolve()
        assert path.is_relative_to(index_path.parent) and sha(path) == case['sha256']
    control = package / 'testdata/control-oracle/native-actions.json.gz'
    assert sha(control) == '3ea26b82248967cc6e98d5b85dee38bdc174603a1d7dd2ddedad1f759bb05225'
    sources = list(package.glob('*.go')) + list((package / 'param').glob('*.go'))
    sources += [ROOT / 'games/yuyuhakusho/data/audio-score.json.gz', ROOT / 'games/yuyuhakusho/data/audio-score-parser-work.json.gz', index_path, control, layout_path, Path(__file__).resolve()]
    bindings = {str(path.relative_to(ROOT)): sha(path) for path in sources}
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'), GOCACHE=str(ROOT / '.local/go-cache'),
               GOMODCACHE=str(ROOT / '.local/go-mod'), GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'),
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
    log = out / (mode + ('.txt' if mode == 'vet' else '.jsonl'))
    start = time.monotonic()
    with log.open('w') as stream:
        code = subprocess.Popen(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT, text=True).wait()
    assert all(sha(ROOT / path) == digest for path, digest in bindings.items()), 'source changed during verification'
    report = {'mode': mode, 'pass': code == 0, 'exit_code': code, 'elapsed_seconds': time.monotonic() - start,
              'command': command, 'environment': {key: env[key] for key in ('TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOOS', 'GOARCH') if key in env},
              'source_sha256': bindings, 'old_tests_catalog_parser_and_42_fixtures_retained': True,
              'persistent_score_state_layout_unchanged': True, 'log_sha256': sha(log)}
    (out / (mode + '-command.json')).write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))
    if code:
        print(log.read_text()[-12000:])
        raise SystemExit(code)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--mode', choices=('native', 'wasm', 'vet'), required=True)
    args = parser.parse_args()
    main(args.out.resolve(), args.mode)
