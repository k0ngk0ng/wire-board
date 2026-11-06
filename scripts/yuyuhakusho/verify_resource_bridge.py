"""Repeat production audio-resource bridge E2E without changing original proofs."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / 'games/yuyuhakusho/verification/audio-resource-bridge'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def test_first():
    first = json.loads((BASE / 'test-first.json').read_text())
    assert first['test_first'] and not first['production_exists'] and not first['enumerator_exists']
    for path, digest in first['files'].items():
        assert sha(ROOT / path) == digest, ('test-first source changed', path)
    assert 'audio/resourcebridge' in (BASE / 'missing-api-red.jsonl').read_text()


def sources():
    files = []
    for name in ['chips', 'chips/psg', 'dsp', 'mixed', 'pcmcommands', 'phaseclock', 'scoreclock', 'score', 'score/param', 'pcmassets', 'resourcebridge']:
        files.extend((ROOT / 'games/yuyuhakusho/audio' / name).glob('*.go'))
    files += [ROOT / 'games/yuyuhakusho/engine/sound_pcm_arbitration.go', Path(__file__).resolve(),
              ROOT / 'scripts/yuyuhakusho/extract_pcm_bridge_intents.py',
              ROOT / 'games/yuyuhakusho/audio/resourcebridge/README.md']
    return {str(p.relative_to(ROOT)): sha(p) for p in sorted(set(files))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', choices=['native', 'wasm', 'vet'])
    parser.add_argument('--out', type=Path)
    parser.add_argument('--audit', action='store_true')
    args = parser.parse_args()
    test_first()
    if args.audit:
        report = json.loads((BASE / 'validation.json').read_text())
        assert report['pass']
        for key in ['sources', 'artifacts']:
            for path, digest in report[key].items():
                assert sha(ROOT / path) == digest, ('frozen binding changed', path)
        print(json.dumps({'pass': True, 'sources': len(report['sources']), 'artifacts': len(report['artifacts'])}))
        return
    assert args.mode and args.out, '--mode and --out are required'
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and out != BASE
    out.mkdir(parents=True, exist_ok=True)
    log = out / (args.mode + ('.txt' if args.mode == 'vet' else '.jsonl'))
    record = out / (args.mode + '-command.json')
    assert not log.exists() and not record.exists(), 'preserve earlier output'
    before = sources()
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'),
               GOCACHE=str(ROOT / '.local/go-cache'), GOMODCACHE=str(ROOT / '.local/go-mod'),
               GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), TMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'),
               GOMAXPROCS='2')
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=10m', '-run', '^TestResourceBridge']
    if args.mode == 'native':
        command.append('-race')
    elif args.mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command += ['-exec', str(Path(goroot) / 'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1')
    else:
        command = [go, 'vet', './games/yuyuhakusho/audio/resourcebridge',
                   './games/yuyuhakusho/audio/pcmassets', './games/yuyuhakusho/audio/chips']
    if args.mode != 'vet':
        command.append('./games/yuyuhakusho/audio/chips')
    start = time.monotonic()
    with log.open('w') as stream:
        result = subprocess.run(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT)
    assert sources() == before, 'sources changed during verification'
    passed = []
    if args.mode != 'vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                row = json.loads(line)
                if row.get('Action') == 'pass' and 'Test' in row:
                    passed.append(row['Test'])
    report = {'schema': 1, 'pass': result.returncode == 0, 'mode': args.mode, 'exit_code': result.returncode,
              'elapsed_seconds': time.monotonic() - start, 'command': command,
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if k in env},
              'sources': before, 'passed_tests': passed, 'log_sha256': sha(log),
              'scope': 'Production resource/config/accepted-command adapter and actual full chip E2E; no independent world/driver timing or platform playback claim'}
    record.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({k: report[k] for k in ['mode', 'pass', 'exit_code', 'elapsed_seconds', 'log_sha256']}))
    if result.returncode:
        print(log.read_text()[-7000:])
        raise SystemExit(result.returncode)


if __name__ == '__main__':
    main()
