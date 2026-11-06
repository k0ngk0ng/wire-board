"""Reproduce actual world-producer and atomic audio transaction checks."""
import argparse
import hashlib
import gzip
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / 'games/yuyuhakusho/verification/world-sound-live'
PATTERN = '^Test(WorldSoundLive|AnimationRecords|CollisionManager|EffectMotion|MatchStart|MatchLifecycle|MatchOverlays|SoundScheduler|SoundRequestLive)'

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def frozen():
    first = json.loads((BASE / 'test-first.json').read_text())
    correction = json.loads((BASE / 'test-adapter-correction.json').read_text())
    assert first['test_first'] and not first['production_exists']
    for path, digest in first['files'].items():
        actual = path
        for item in [correction, correction['startup']]:
            if path == item['current_test']:
                actual = item['original_test_archive']
        assert sha(ROOT / actual) == digest, actual
    for item in [correction, correction['startup']]:
        assert sha(ROOT / item['current_test']) == item['current_sha256']
    assert 'undefined: StepChuWorldObjectsObserved' in (BASE / 'missing-api-red.jsonl').read_text()

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--mode', choices=['native', 'native-race', 'wasm', 'vet'])
    p.add_argument('--out', type=Path)
    p.add_argument('--audit', action='store_true')
    args = p.parse_args()
    frozen()
    if args.audit:
        manifest = json.loads((BASE / 'validation.json').read_text())
        assert manifest['pass']
        for section in ['sources', 'artifacts']:
            for path, digest in manifest[section].items():
                assert sha(ROOT / path) == digest, path
        for path, metadata in manifest['compressed_logs'].items():
            raw = gzip.decompress((ROOT / path).read_bytes())
            assert len(raw) == metadata['raw_bytes'] and hashlib.sha256(raw).hexdigest() == metadata['raw_sha256'], path
        print(json.dumps({'pass': True, 'sources': len(manifest['sources']), 'artifacts': len(manifest['artifacts'])}))
        return
    assert args.mode and args.out
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and not out.exists(), 'use a fresh workspace output directory'
    out.mkdir(parents=True)
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'), GOCACHE=str(ROOT / '.local/go-cache'),
               GOMODCACHE=str(ROOT / '.local/go-mod'), GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=20m', '-run', PATTERN]
    if args.mode == 'native-race':
        command = [go, 'test', '-json', '-count=1', '-timeout=5m', '-race', '-run', '^TestWorldSoundLive(Interleaving|Rollback|FullQueue|Joint)']
    elif args.mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], env=env, cwd=ROOT, text=True).strip()
        command += ['-exec', str(Path(goroot) / 'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1', GODEBUG='gccheckmark=1,clobberfree=1')
    command += ['./games/yuyuhakusho/engine', './games/yuyuhakusho/audio/scoreclock']
    if args.mode == 'vet':
        command = [go, 'vet', './games/yuyuhakusho/engine', './games/yuyuhakusho/audio/session', './games/yuyuhakusho/audio/scoreclock']
    start = time.time()
    with (out / 'output.jsonl').open('w') as log:
        proc = subprocess.run(command, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
    record = {'command': command, 'cwd': str(ROOT), 'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH', 'GODEBUG'] if k in env},
              'elapsed_seconds': time.time()-start, 'exit_code': proc.returncode, 'output_sha256': sha(out / 'output.jsonl')}
    if args.mode != 'vet':
        records = [json.loads(line) for line in (out / 'output.jsonl').read_text().splitlines()]
        record['tests_passed'] = sum(r['Action'] == 'pass' and 'Test' in r for r in records)
        record['top_level_passed'] = sum(r['Action'] == 'pass' and 'Test' in r and '/' not in r['Test'] for r in records)
        record['failures'] = [r for r in records if r['Action'] in ['fail', 'build-fail', 'skip']]
    (out / 'command.json').write_text(json.dumps(record, indent=2)+'\n')
    print(json.dumps(record))
    raise SystemExit(proc.returncode)

if __name__ == '__main__':
    main()
