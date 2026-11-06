"""Verify complete fight controls and independent FM/PSG integration."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_dispatch_init_clock as common

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT/'games/yuyuhakusho/verification/score-clock-driver-control'
LANES = ROOT/'games/yuyuhakusho/verification/fmpsg-independent-lanes'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    correction = json.loads((BASE/'test-call-count-correction.json').read_text())
    for base in [BASE, LANES]:
        first = json.loads((base/'test-first.json').read_text())
        assert first['test_first'] and not first['production_exists']
        for path, digest in first['files'].items():
            source = ROOT/path
            if path == correction['current_test']:
                source = ROOT/correction['original_test_archive']
            assert sha(source) == digest, ('frozen input changed', path)
    assert sha(ROOT/correction['current_test']) == correction['current_test_sha256']
    for path, digest in correction['baseline_contract_sources'].items():
        assert sha(ROOT/path) == digest
    assert not correction['native_oracle_changed'] and not correction['production_changed_for_test_correction']
    assert 'ReadPauseLevels undefined' in (BASE/'missing-api-red.jsonl').read_text()
    assert 'WritePortOnLane undefined' in (LANES/'missing-api-red.jsonl').read_text()


def sources():
    result = common.sources()
    files = [Path(__file__).resolve(), ROOT/'scripts/yuyuhakusho/score-clock-driver-control-capture.py',
             ROOT/'scripts/yuyuhakusho/fmpsg-independent-lanes-capture.py', LANES/'native.json.gz',
             LANES/'native-capture.json', ROOT/'games/yuyuhakusho/audio/chips/INDEPENDENT_LANES.md',
             ROOT/'games/yuyuhakusho/audio/scoreclock/DRIVER_CONTROL.md', BASE/'case-export-source.go.txt']
    for path in files:
        result[str(path.relative_to(ROOT))] = sha(path)
    return dict(sorted(result.items()))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', choices=['native-clock', 'wasm-clock', 'native-chips', 'wasm-chips', 'vet'])
    parser.add_argument('--out', type=Path)
    parser.add_argument('--audit', action='store_true')
    args = parser.parse_args()
    frozen()
    if args.audit:
        report = json.loads((BASE/'validation.json').read_text())
        assert report['pass'] and sources() == report['sources']
        for key in ['sources', 'artifacts']:
            for path, digest in report[key].items():
                assert sha(ROOT/path) == digest, ('binding changed', path)
        for path, meta in report['compressed_logs'].items():
            raw = gzip.decompress((ROOT/path).read_bytes())
            assert len(raw) == meta['raw_bytes'] and hashlib.sha256(raw).hexdigest() == meta['raw_sha256']
        print(json.dumps({'pass': True, 'sources': len(report['sources']), 'artifacts': len(report['artifacts'])}))
        return
    assert args.mode and args.out
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and out not in [BASE, LANES]
    out.mkdir(parents=True, exist_ok=True)
    record = out/(args.mode+'-command.json')
    log = out/(args.mode+('.txt' if args.mode == 'vet' else '.jsonl'))
    assert not record.exists() and not log.exists(), 'preserve prior checks'
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'), GOCACHE=str(ROOT/'.local/go-cache'),
               GOMODCACHE=str(ROOT/'.local/go-mod'), GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=12m']
    if args.mode.startswith('native'):
        command.append('-race')
    elif args.mode.startswith('wasm'):
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command += ['-exec', str(Path(goroot)/'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1')
    if args.mode.endswith('clock'):
        command += ['./games/yuyuhakusho/audio/scoreclock', './games/yuyuhakusho/audio/rootcontrol']
    elif args.mode.endswith('chips'):
        command += ['./games/yuyuhakusho/audio/chips']
    else:
        command = [go, 'vet']+['./games/yuyuhakusho/audio/'+p for p in ['scoreclock', 'rootcontrol', 'score', 'chips', 'mixed']]
    before, start = sources(), time.monotonic()
    with log.open('w') as stream:
        run = subprocess.run(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT)
    assert sources() == before, 'source changed during verification'
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
              'exit_code': run.returncode, 'elapsed_seconds': time.monotonic()-start, 'command': command,
              'sources': before, 'passed_tests': passed, 'skipped_tests': skipped, 'log_sha256': sha(log),
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR',
                                                'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if k in env},
              'scope': '103 original whole control/fade/pause profiles; real Root/score/mixed integration and checkpoint continuation. '
                       'Eight original full YM/PSG/FIR/PCM independent-lane cases and all chip/clock/Root consumers. '
                       'World IRQ, request production, joint platform checkpoint and full browser audio remain incomplete.'}
    record.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({k: report[k] for k in ['mode', 'pass', 'exit_code', 'elapsed_seconds']}))
    if not report['pass']:
        print(log.read_text()[-6500:])
        raise SystemExit(run.returncode or 1)


if __name__ == '__main__':
    main()
