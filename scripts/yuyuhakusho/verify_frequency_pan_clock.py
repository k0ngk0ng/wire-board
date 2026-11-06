"""Verify prepared frequency writes, pan automation and scoreclock regressions."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_observed_envelope_key as common

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT/'games/yuyuhakusho/verification/score-clock-frequency-pan'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first = json.loads((BASE/'test-first.json').read_text())
    assert first['test_first'] and not first['production_exists']
    for path, digest in first['files'].items():
        assert sha(ROOT/path) == digest, ('frozen test/oracle changed', path)
    assert 'undefined: scoreclock.BeginFrequencyTransfer' in (BASE/'missing-api-red.jsonl').read_text()


def sources():
    result = common.sources()
    for path in [Path(__file__).resolve(), ROOT/'scripts/yuyuhakusho/score-clock-frequency-pan-capture.py',
                 ROOT/'scripts/yuyuhakusho/score-clock-pan-capture.py',
                 ROOT/'scripts/yuyuhakusho/score-clock-psg-capture.py',
                 ROOT/'scripts/yuyuhakusho/score_parameter_tables.py',
                 ROOT/'games/yuyuhakusho/audio/scoreclock/FREQUENCY_PAN.md']:
        result[str(path.relative_to(ROOT))] = sha(path)
    return dict(sorted(result.items()))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', choices=['native', 'wasm', 'vet'])
    parser.add_argument('--out', type=Path)
    parser.add_argument('--audit', action='store_true')
    args = parser.parse_args()
    frozen()
    if args.audit:
        report = json.loads((BASE/'validation.json').read_text())
        assert report['pass']
        for key in ['sources', 'artifacts']:
            for path, digest in report[key].items():
                assert sha(ROOT/path) == digest, ('binding changed', path)
        print(json.dumps({'pass': True, 'sources': len(report['sources']), 'artifacts': len(report['artifacts'])}))
        return
    assert args.mode and args.out
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and out != BASE
    out.mkdir(parents=True, exist_ok=True)
    record = out/(args.mode+'-command.json')
    log = out/(args.mode+('.txt' if args.mode == 'vet' else '.jsonl'))
    assert not record.exists() and not log.exists(), 'preserve prior verification'
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'), GOCACHE=str(ROOT/'.local/go-cache'),
               GOMODCACHE=str(ROOT/'.local/go-mod'), GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=5m']
    if args.mode == 'native':
        command.append('-race')
    elif args.mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command += ['-exec', str(Path(goroot)/'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1')
    command.append('./games/yuyuhakusho/audio/scoreclock')
    if args.mode == 'vet':
        command = [go, 'vet', './games/yuyuhakusho/audio/scoreclock', './games/yuyuhakusho/audio/score']
    before, start = sources(), time.monotonic()
    with log.open('w') as stream:
        result = subprocess.run(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT)
    assert before == sources(), 'source changed during verification'
    passed, skipped = [], []
    if args.mode != 'vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                item = json.loads(line)
                if 'Test' in item and item.get('Action') == 'pass':
                    passed.append(item['Test'])
                if 'Test' in item and item.get('Action') == 'skip':
                    skipped.append(item['Test'])
    report = {'schema': 1, 'mode': args.mode, 'pass': result.returncode == 0 and not skipped,
              'exit_code': result.returncode, 'command': command, 'elapsed_seconds': time.monotonic()-start,
              'sources': before, 'passed_tests': passed, 'skipped_tests': skipped, 'log_sha256': sha(log),
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR',
                                                'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if k in env},
              'scope': '128 original frequency/pan profiles and actual score-pan state ownership; complete scoreclock regression. Full voice/world audio remains incomplete.'}
    record.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({k: report[k] for k in ['mode', 'pass', 'exit_code', 'elapsed_seconds']}))
    if not report['pass']:
        print(log.read_text()[-6500:])
        raise SystemExit(result.returncode or 1)


if __name__ == '__main__':
    main()
