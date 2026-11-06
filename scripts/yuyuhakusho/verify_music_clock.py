"""Repeat original music publication with actual resources and scoreclock regressions."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT/'games/yuyuhakusho/verification/score-clock-music'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first = json.loads((BASE/'test-first.json').read_text())
    assert first['test_first'] and not first['production_exists']
    for path, digest in first['files'].items():
        assert sha(ROOT/path) == digest, ('test-first source changed', path)
    assert 'undefined: scoreclock.MusicRequest' in (BASE/'missing-api-red.jsonl').read_text()


def sources():
    files = []
    for package in ['scoreclock', 'rootcontrol', 'score', 'score/param', 'mixed', 'pcmcommands',
                    'phaseclock', 'chips', 'chips/psg', 'dsp', 'resourcebridge', 'pcmassets']:
        files.extend((ROOT/'games/yuyuhakusho/audio'/package).glob('*.go'))
    files += list((ROOT/'games/yuyuhakusho/audio/scoreclock/testdata').glob('*'))
    files += list((ROOT/'games/yuyuhakusho/engine').glob('sound*.go'))
    for folder in (ROOT/'games/yuyuhakusho/verification').glob('score-clock-*'):
        files += list(folder.glob('native*.gz'))
    for name in ['audio-score', 'audio-pcm-payloads', 'audio-score-parser-work', 'audio-score-instrument-work']:
        files.append(ROOT/'games/yuyuhakusho/data'/(name+'.json.gz'))
    files += [Path(__file__).resolve(), ROOT/'scripts/yuyuhakusho/score-clock-music-capture.py',
              ROOT/'scripts/yuyuhakusho/score-clock-music-observer.c', ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',
              ROOT/'games/yuyuhakusho/audio/scoreclock/MUSIC.md',
              ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']
    return {str(p.relative_to(ROOT)): sha(p) for p in sorted(set(files)) if p.is_file()}


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
    log = out/(args.mode+('.txt' if args.mode == 'vet' else '.jsonl'))
    record = out/(args.mode+'-command.json')
    assert not log.exists() and not record.exists(), 'preserve prior checks'
    before = sources()
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
    else:
        command = [go, 'vet']
    command.append('./games/yuyuhakusho/audio/scoreclock')
    start = time.monotonic()
    with log.open('w') as stream:
        result = subprocess.run(command, cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT)
    assert before == sources(), 'source changed during verification'
    passed, skipped = [], []
    if args.mode != 'vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                row = json.loads(line)
                if 'Test' in row:
                    if row.get('Action') == 'pass':
                        passed.append(row['Test'])
                    elif row.get('Action') == 'skip':
                        skipped.append(row['Test'])
    report = {'schema': 1, 'mode': args.mode, 'pass': result.returncode == 0 and not skipped,
              'exit_code': result.returncode, 'command': command, 'elapsed_seconds': time.monotonic()-start,
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR',
                                                'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH'] if k in env},
              'sources': before, 'passed_tests': passed, 'skipped_tests': skipped, 'log_sha256': sha(log),
              'scope': 'Complete music DAC commit/arbitration/publication with actual four-resource mixed Runtime plus scoreclock regressions; full world audio remains incomplete'}
    record.write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({k: report[k] for k in ['mode', 'pass', 'exit_code', 'elapsed_seconds', 'log_sha256']}))
    if not report['pass']:
        print(log.read_text()[-7000:])
        raise SystemExit(result.returncode or 1)


if __name__ == '__main__':
    main()
