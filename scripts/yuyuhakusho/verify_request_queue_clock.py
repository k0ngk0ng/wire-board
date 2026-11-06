"""Verify live queue consumption, ordinary dispatch and current Root/clock consumers."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_running_tick_clock as common

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT/'games/yuyuhakusho/verification/score-clock-request-queue'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first = json.loads((BASE/'test-first.json').read_text())
    assert first['test_first'] and not first['production_exists']
    for path, digest in first['files'].items():
        assert sha(ROOT/path) == digest, ('frozen input changed', path)
    red = (BASE/'missing-api-red.jsonl').read_text()
    for missing in ['scoreclock.ConsumeQueue', 'scoreclock.AdvanceRequestTick', 'ConsumeSoundQueueObserved']:
        assert 'undefined: '+missing in red


def sources():
    result = common.sources()
    files = [Path(__file__).resolve(), ROOT/'scripts/yuyuhakusho/score-clock-request-queue-capture.py',
             ROOT/'games/yuyuhakusho/audio/scoreclock/REQUEST_QUEUE.md']
    files += list((ROOT/'games/yuyuhakusho/engine').glob('sound_queue*.go'))
    files += list((ROOT/'games/yuyuhakusho/engine/testdata').glob('sound-queue-*'))
    for path in files:
        if path.is_file():
            result[str(path.relative_to(ROOT))] = sha(path)
    return dict(sorted(result.items()))


def audit():
    report = json.loads((BASE/'validation.json').read_text())
    assert report['pass']
    assert sources() == report['sources'], 'current source set differs'
    for key in ['sources', 'artifacts']:
        for path, digest in report[key].items():
            assert sha(ROOT/path) == digest, ('binding changed', path)
    for name, meta in report['compressed_logs'].items():
        raw = gzip.decompress((BASE/name).read_bytes())
        assert len(raw) == meta['raw_bytes']
        assert hashlib.sha256(raw).hexdigest() == meta['raw_sha256']
    print(json.dumps({'pass': True, 'sources': len(report['sources']), 'artifacts': len(report['artifacts'])}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', choices=['native', 'wasm', 'engine-native', 'engine-wasm', 'vet'])
    parser.add_argument('--out', type=Path)
    parser.add_argument('--audit', action='store_true')
    args = parser.parse_args()
    frozen()
    if args.audit:
        audit()
        return
    assert args.mode and args.out
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and out != BASE
    out.mkdir(parents=True, exist_ok=True)
    log = out/(args.mode+('.txt' if args.mode == 'vet' else '.jsonl'))
    record = out/(args.mode+'-command.json')
    assert not log.exists() and not record.exists(), 'preserve prior checks'
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'), GOCACHE=str(ROOT/'.local/go-cache'),
               GOMODCACHE=str(ROOT/'.local/go-mod'), GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    go = shutil.which('go')
    assert go
    # The previous native clock/Root run took 409 s. Allow the additional request
    # scope without turning a fixed wall-clock cap into an unrelated failure.
    command = [go, 'test', '-json', '-count=1', '-timeout=12m']
    if args.mode.endswith('native'):
        command.append('-race')
    elif args.mode.endswith('wasm'):
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command += ['-exec', str(Path(goroot)/'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1')
    if args.mode.startswith('engine-'):
        command += ['-run', '^TestSoundQueue', './games/yuyuhakusho/engine']
    else:
        command += ['./games/yuyuhakusho/audio/scoreclock', './games/yuyuhakusho/audio/rootcontrol']
    if args.mode == 'vet':
        command = [go, 'vet', './games/yuyuhakusho/audio/scoreclock',
                   './games/yuyuhakusho/audio/rootcontrol', './games/yuyuhakusho/engine']
    before, start = sources(), time.monotonic()
    with log.open('w') as stream:
        run = subprocess.run(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT)
    assert before == sources(), 'source changed during verification'
    passed, skipped = [], []
    if args.mode != 'vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                row = json.loads(line)
                if 'Test' in row and row.get('Action') == 'pass':
                    passed.append({'package': row['Package'], 'test': row['Test']})
                if 'Test' in row and row.get('Action') == 'skip':
                    skipped.append({'package': row['Package'], 'test': row['Test']})
    report = {'schema': 1, 'mode': args.mode, 'pass': run.returncode == 0 and not skipped,
              'exit_code': run.returncode, 'command': command, 'elapsed_seconds': time.monotonic()-start,
              'sources': before, 'passed_tests': passed, 'skipped_tests': skipped, 'log_sha256': sha(log),
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE',
                                                'GOTMPDIR', 'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if k in env},
              'scope': '287 original live queue profiles and 122 complete ordinary PCM request ticks; '
                       'full scoreclock/Root regression, engine queue tests selected separately. '
                       'Music/primary/control dispatch, active fade/pause, world IRQ and platform audio remain incomplete.'}
    record.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({k: report[k] for k in ['mode', 'pass', 'exit_code', 'elapsed_seconds']}))
    if not report['pass']:
        print(log.read_text()[-6500:])
        raise SystemExit(run.returncode or 1)


if __name__ == '__main__':
    main()
