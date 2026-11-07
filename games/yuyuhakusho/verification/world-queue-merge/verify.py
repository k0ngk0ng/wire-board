#!/usr/bin/env python3
"""Reproduce the bounded live queue merge and retained original regressions."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

BASE = Path(__file__).resolve().parent
ROOT = BASE.parents[3]
ENGINE = ROOT/'games/yuyuhakusho/engine'
FILES = ['world_sound_live.go', 'chu_actor.go', 'chu_action.go', 'chu_tail.go',
         'fighter4_world.go', 'fighter5_world.go', 'fighter7_world.go',
         'world_queue_merge_test.go']
PATTERN = '^Test(WorldQueueMerge|WorldSoundLive|ChuActor(OriginalRules|LateFailureIsAtomic)|ChuAction(OriginalCompleteCalls|ErrorsAreAtomic)|ChuTail(OriginalFullCalls|ErrorsAreAtomic)|Fighter4DragonWorldCallbacks|Fighter5WorldChildCallbacks|Fighter7WorldChildCallbacks)'
RACE = '^Test(WorldQueueMerge|WorldSoundLive(Interleaving|Rollback|FullQueue|Joint))'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--mode', choices=['native', 'native-race', 'wasm', 'vet'], required=True)
    p.add_argument('--out', type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    assert out.is_relative_to(BASE) and not out.exists()
    out.mkdir(parents=True)
    frozen = json.loads((BASE/'test-first.json').read_bytes())
    assert frozen['helper_exists'] is False
    for path, digest in frozen['frozen'].items():
        assert sha(ROOT/path) == digest, ('frozen test changed', path)
    assert 'undefined: worldSoundQueueView' in (BASE/'missing-helper-red.jsonl').read_text()
    assert 'no-enqueue component restored stale queue' in (BASE/'stale-component-red.jsonl').read_text()
    assert 'outer return resurrected the last consumed enqueue' in (BASE/'stale-component-red.jsonl').read_text()
    paths = [ENGINE/name for name in FILES]+[Path(__file__)]
    sources = {str(path.relative_to(ROOT)): sha(path) for path in paths}
    shared = [ENGINE/'chu_world.go', ENGINE/'arena_work.go', ENGINE/'chu_session.go',
              ROOT/'games/yuyuhakusho/audio/session/world.go']
    shared_before = {str(path.relative_to(ROOT)): sha(path) for path in shared}
    env = dict(os.environ, TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),
               GOCACHE=str(ROOT/'.local/go-cache'), GOMODCACHE=str(ROOT/'.local/go-mod'),
               GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),
               TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'), GOMAXPROCS='2')
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=20m', '-run', RACE if a.mode == 'native-race' else PATTERN]
    if a.mode == 'native-race':
        command += ['-race']
    elif a.mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command += ['-exec', str(Path(goroot)/'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1', GODEBUG='gccheckmark=1,clobberfree=1')
    command += ['./games/yuyuhakusho/engine', './games/yuyuhakusho/audio/scoreclock']
    if a.mode == 'vet':
        command = [go, 'vet', './games/yuyuhakusho/engine', './games/yuyuhakusho/audio/session', './games/yuyuhakusho/audio/scoreclock']
    start = time.monotonic()
    with (out/'output.jsonl').open('w') as log:
        completed = subprocess.run(command, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
    result = {'pass': completed.returncode == 0, 'mode': a.mode, 'command': command, 'cwd': str(ROOT),
              'exit_code': completed.returncode, 'elapsed_seconds': time.monotonic()-start,
              'environment': {k: env[k] for k in ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH', 'GODEBUG'] if k in env},
              'output_sha256': sha(out/'output.jsonl'), 'sources': sources,
              'test_first_sha256': sha(BASE/'test-first.json'),
              'shared_sources_before': shared_before,
              'shared_sources_after': {str(path.relative_to(ROOT)): sha(path) for path in shared}}
    result['shared_sources_stable'] = result['shared_sources_before'] == result['shared_sources_after']
    result['owned_sources_stable'] = all(sha(ROOT/path) == digest for path, digest in sources.items())
    if a.mode != 'vet':
        rows = [json.loads(line) for line in (out/'output.jsonl').read_text().splitlines() if line.startswith('{')]
        result['tests_passed'] = sum(r['Action'] == 'pass' and 'Test' in r for r in rows)
        result['top_level_passed'] = sum(r['Action'] == 'pass' and 'Test' in r and '/' not in r['Test'] for r in rows)
        result['failures'] = [r for r in rows if r['Action'] in ['fail', 'build-fail', 'skip']]
        result['pass'] &= not result['failures'] and result['tests_passed'] > 0
    result['pass'] &= result['owned_sources_stable']
    (out/'command.json').write_text(json.dumps(result, indent=2)+'\n')
    print(json.dumps({k: result[k] for k in ['pass', 'mode', 'exit_code', 'elapsed_seconds', 'owned_sources_stable', 'shared_sources_stable']}))
    raise SystemExit(0 if result['pass'] else 1)


if __name__ == '__main__':
    main()
