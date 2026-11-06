"""Verify actual observed score-to-PCM stages and bounded legacy compatibility."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_music_clock as common

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT/'games/yuyuhakusho/verification/score-observed-pcm'
ORIGINAL_CASES = ['music82-a0', 'music82-a1', 'higher-priority-replaces', 'lower-priority-rejected']


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first = json.loads((BASE/'test-first.json').read_text())
    fix = json.loads((BASE/'test-setup-correction.json').read_text())
    assert first['test_first'] and not first['production_exists'] and fix['new_api_not_implemented']
    for path, digest in first['files'].items():
        if path == fix['current_test']:
            assert sha(ROOT/fix['original_test_archive']) == digest == fix['before_sha256']
            assert sha(ROOT/path) == fix['after_sha256']
            before = (ROOT/fix['original_test_archive']).read_text()
            after = (ROOT/path).read_text()
            start = 'func TestObservedPCMActualAutomaticPanStages'
            end = 'func TestObservedPCMPanCallValueOriginalAndDetached'
            assert before.split(start)[0] == after.split(start)[0]
            assert before.split(end)[1] == after.split(end)[1]
        else:
            assert sha(ROOT/path) == digest, ('frozen input changed', path)
    assert sha(ROOT/'games/yuyuhakusho/data/audio-score.json.gz') == fix['source_catalog_sha256']
    assert 'read.TrackPan undefined' in (BASE/'missing-api-red.jsonl').read_text()
    assert 'undefined: scoreclock.BeginObservedPan' in (BASE/'direct-pan-missing-api-red.jsonl').read_text()


def sources():
    result = common.sources()
    paths = [Path(__file__).resolve(), ROOT/'scripts/yuyuhakusho/score-clock-pan-value-capture.py',
             ROOT/'scripts/yuyuhakusho/score-clock-pan-capture.py', ROOT/'scripts/yuyuhakusho/score-clock-pan-observer.c',
             ROOT/'games/yuyuhakusho/audio/scoreclock/OBSERVED_PCM.md', BASE/'native-pan-value-full.json.gz']
    data = ROOT/'games/yuyuhakusho/audio/score/testdata'
    index = data/'original-index.json'
    paths.append(index)
    for row in json.loads(index.read_text())['cases']:
        if row['id'] in ORIGINAL_CASES:
            paths.append(data/row['path'])
    result.update({str(p.relative_to(ROOT)): sha(p) for p in paths})
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
                assert sha(ROOT/path) == digest, ('frozen binding changed', path)
        print(json.dumps({'pass': True, 'sources': len(report['sources']), 'artifacts': len(report['artifacts'])}))
        return
    assert args.mode and args.out
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and out != BASE
    out.mkdir(parents=True, exist_ok=True)
    record = out/(args.mode+'-command.json')
    assert not record.exists(), 'preserve earlier verification'
    env = os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'), GOCACHE=str(ROOT/'.local/go-cache'),
               GOMODCACHE=str(ROOT/'.local/go-mod'), GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    go = shutil.which('go')
    assert go
    before = sources()
    flags = [go, 'test', '-json', '-count=1', '-timeout=5m']
    if args.mode == 'native':
        flags.append('-race')
    elif args.mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        flags += ['-exec', str(Path(goroot)/'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1')
    checks = [('clock', flags+['./games/yuyuhakusho/audio/scoreclock']),
              ('score-observed', flags+['-run', '^Test(Observed(TempoWorkComesFromCurrentState|FeedbackReadsAreStaged|PriorityWritesAreAtOriginalPositions|ErrorsDoNotCommitScoreOrRepeatCallbacks|RecordsHaveNoMutableOutputAlias)|TickTransaction.*|MusicResetHeaderEffectsAreExplicitAndOrdered)$', './games/yuyuhakusho/audio/score']),
              ('score-original', flags+['-run', '^TestObservedOriginalTrajectories$/^('+'|'.join(ORIGINAL_CASES)+')$', './games/yuyuhakusho/audio/score'])]
    if args.mode == 'vet':
        checks = [('vet', [go, 'vet']+['./games/yuyuhakusho/audio/'+p for p in ['scoreclock', 'score', 'mixed', 'pcmcommands']])]
    results = []
    start = time.monotonic()
    for label, command in checks:
        log = out/(args.mode+'-'+label+('.txt' if args.mode == 'vet' else '.jsonl'))
        assert not log.exists(), 'preserve earlier log'
        with log.open('w') as stream:
            result = subprocess.run(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT)
        passed, skipped = [], []
        if args.mode != 'vet':
            for line in log.read_text().splitlines():
                if line.startswith('{'):
                    row = json.loads(line)
                    if 'Test' in row and row.get('Action') == 'pass': passed.append(row['Test'])
                    if 'Test' in row and row.get('Action') == 'skip': skipped.append(row['Test'])
        results.append({'label': label, 'command': command, 'exit_code': result.returncode,
                        'pass': result.returncode == 0 and not skipped, 'passed_tests': passed,
                        'skipped_tests': skipped, 'log_sha256': sha(log)})
        if not results[-1]['pass']:
            print(log.read_text()[-6000:])
            break
    assert sources() == before, 'source changed during verification'
    report = {'schema': 1, 'mode': args.mode, 'pass': all(r['pass'] for r in results),
              'elapsed_seconds': time.monotonic()-start, 'sources': before, 'checks': results,
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if k in env},
              'original_cases': ORIGINAL_CASES, 'original_frames': 546,
              'scope': 'Actual score music/ordinary-pan phase connection, full scoreclock and bounded observed/transaction/native legacy compatibility; whole tick/world playback remains unpriced'}
    record.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({'mode': args.mode, 'pass': report['pass'], 'elapsed_seconds': report['elapsed_seconds'],
                      'checks': [{'label': r['label'], 'pass': r['pass'], 'records': len(r['passed_tests'])} for r in results]}))
    if not report['pass']: raise SystemExit(1)


if __name__ == '__main__':
    main()
