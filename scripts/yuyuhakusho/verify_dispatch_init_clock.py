"""Verify whole music/primary dispatch and bounded score/Root compatibility."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_request_queue_clock as common

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT/'games/yuyuhakusho/verification/score-clock-dispatch-init'
ORIGINAL_CASES = [f'music82-{code:02x}' for code in range(0xa0, 0xac)] + [
    'higher-priority-replaces', 'lower-priority-rejected', 'music-replaces-primary']


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first = json.loads((BASE/'test-first.json').read_text())
    assert first['test_first'] and not first['production_exists']
    for path, digest in first['files'].items():
        assert sha(ROOT/path) == digest, ('frozen input changed', path)
    red = (BASE/'missing-api-red.jsonl').read_text()
    assert 'undefined: scoreclock.AdvanceDriverTick' in red and 'unknown field HeaderEffect' in red


def sources():
    # Native fixtures are inputs; generated JSONL verification logs are outputs.
    result = {p: digest for p, digest in common.sources().items() if not p.endswith('.jsonl.gz')}
    files = [Path(__file__).resolve(), ROOT/'scripts/yuyuhakusho/score-clock-dispatch-init-capture.py',
             ROOT/'games/yuyuhakusho/audio/scoreclock/DISPATCH_INIT.md',
             ROOT/'docs/research/md-yuyuhakusho/static/audio-control.txt', BASE/'case-export-source.go.txt']
    index = ROOT/'games/yuyuhakusho/audio/score/testdata/original-index.json'
    doc = json.loads(index.read_text())
    files.append(index)
    for row in doc['cases']:
        if row['id'] in ORIGINAL_CASES:
            path = index.parent/row['path']
            assert sha(path) == row['sha256']
            files.append(path)
    for path in files:
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
        r = json.loads((BASE/'validation.json').read_text())
        assert r['pass'] and sources() == r['sources']
        for key in ['sources', 'artifacts']:
            for path, digest in r[key].items():
                assert sha(ROOT/path) == digest, ('binding changed', path)
        for name, meta in r['compressed_logs'].items():
            raw = gzip.decompress((BASE/name).read_bytes())
            assert len(raw) == meta['raw_bytes'] and hashlib.sha256(raw).hexdigest() == meta['raw_sha256']
        print(json.dumps({'pass': True, 'sources': len(r['sources']), 'artifacts': len(r['artifacts'])}))
        return
    assert args.mode and args.out
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and out != BASE
    out.mkdir(parents=True, exist_ok=True)
    record = out/(args.mode+'-command.json')
    assert not record.exists(), 'preserve prior checks'
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'), GOCACHE=str(ROOT/'.local/go-cache'),
               GOMODCACHE=str(ROOT/'.local/go-mod'), GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    go = shutil.which('go')
    assert go
    flags = [go, 'test', '-json', '-count=1', '-timeout=12m']
    if args.mode == 'native':
        flags.append('-race')
    elif args.mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        flags += ['-exec', str(Path(goroot)/'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1')
    checks = [('clock-root', flags+['./games/yuyuhakusho/audio/scoreclock', './games/yuyuhakusho/audio/rootcontrol']),
              ('score-contract', flags+['-run', '^Test(TickTransaction.*|MusicResetHeaderEffectsAreExplicitAndOrdered|Observed(TempoWorkComesFromCurrentState|FeedbackReadsAreStaged|PriorityWritesAreAtOriginalPositions|ErrorsDoNotCommitScoreOrRepeatCallbacks|RecordsHaveNoMutableOutputAlias)|ControlReview.*|OwnedSnapshotsAndAtomicRejection)$', './games/yuyuhakusho/audio/score']),
              ('score-original', flags+['-run', '^Test(OriginalContinuousDriver|ObservedOriginalTrajectories)$/^('+'|'.join(ORIGINAL_CASES)+')$', './games/yuyuhakusho/audio/score'])]
    if args.mode == 'vet':
        checks = [('vet', [go, 'vet', './games/yuyuhakusho/audio/scoreclock',
                           './games/yuyuhakusho/audio/score', './games/yuyuhakusho/audio/rootcontrol'])]
    before, start, results = sources(), time.monotonic(), []
    for label, command in checks:
        log = out/(args.mode+'-'+label+('.txt' if args.mode == 'vet' else '.jsonl'))
        assert not log.exists()
        with log.open('w') as stream:
            run = subprocess.run(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT)
        passed, skipped = [], []
        if args.mode != 'vet':
            for line in log.read_text().splitlines():
                if line.startswith('{'):
                    row = json.loads(line)
                    if 'Test' in row and row.get('Action') == 'pass':
                        passed.append({'package': row['Package'], 'test': row['Test']})
                    if 'Test' in row and row.get('Action') == 'skip':
                        skipped.append(row['Test'])
        results.append({'label': label, 'pass': run.returncode == 0 and not skipped, 'exit_code': run.returncode,
                        'command': command, 'passed_tests': passed, 'skipped_tests': skipped, 'log_sha256': sha(log)})
        if not results[-1]['pass']:
            print(log.read_text()[-6500:])
            break
    assert before == sources(), 'source changed during verification'
    report = {'schema': 1, 'mode': args.mode, 'pass': all(r['pass'] for r in results),
              'elapsed_seconds': time.monotonic()-start, 'sources': before, 'checks': results,
              'original_cases': ORIGINAL_CASES,
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR',
                                                'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if k in env},
              'scope': '63 whole original music/primary dispatch calls, complete clock/Root regression, '
                       'score contracts and 15 bounded original streams through both interfaces. '
                       'Control/fade/pause, world IRQ and platform audio remain incomplete.'}
    record.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'mode': args.mode, 'pass': report['pass'], 'elapsed_seconds': report['elapsed_seconds'],
                      'checks': [{'label': x['label'], 'pass': x['pass'], 'records': len(x['passed_tests'])} for x in results]}))
    if not report['pass']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
