"""Retain reproducible cold-configuration Go verification and first-boundary state."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[2]
TEST = 'TestColdConfiguredWorldSixThousandUpdates'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--race', action='store_true')
    parser.add_argument('--wasm', action='store_true')
    parser.add_argument('--all-computer', action='store_true', help='independent four-computer configuration; keeps the mixed test unchanged')
    parser.add_argument('--output')
    args = parser.parse_args()
    if args.race and args.wasm:
        parser.error('Go race detection is not available for Wasm')
    test = 'TestAllComputerColdWorldSixThousandUpdates' if args.all_computer else TEST
    default_output = '.local/basketball/rea/all-computer-cold' if args.all_computer else '.local/basketball/rea/cold-runtime'
    output = (ROOT / (args.output or default_output)).resolve()
    if not output.is_relative_to(ROOT):
        parser.error('output must remain inside the workspace')
    output.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    for key, relative in [('GOCACHE', '.local/go-build'), ('GOMODCACHE', '.local/go-mod'), ('GOTMPDIR', '.local/tmp')]:
        env[key] = str(ROOT / relative)
        Path(env[key]).mkdir(parents=True, exist_ok=True)
    env['TMPDIR'] = env['GOTMPDIR']
    env['GOTOOLCHAIN'] = 'local'
    command = ['go', 'test', '-json', './games/basketball/engine', '-run', '^'+test+'$', '-count=1']
    flavor = 'native'
    if args.race:
        command.append('-race')
        flavor = 'race'
    if args.wasm:
        goroot = subprocess.check_output(['go', 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command.extend(['-exec', str(Path(goroot) / 'lib/wasm/go_js_wasm_exec')])
        env.update(GOOS='js', GOARCH='wasm')
        flavor = 'wasm'
    result = subprocess.run(command, cwd=ROOT, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    log = output / (flavor+'-tests.jsonl')
    log.write_bytes(result.stdout)
    events = []
    for line in result.stdout.decode(errors='replace').splitlines():
        try:
            events.append(json.loads(line))
        except json.JSONDecodeError:
            pass  # Compiler stderr remains intact in the raw log.
    transcript = ''.join(e.get('Output', '') for e in events)
    failure = re.search(r'cold seed 12345678, update (\d+)/(\d+): ([^\n]+)', transcript)
    before = re.search(r'^\s*before=(.+)$', transcript, re.MULTILINE)
    passed = any(e.get('Action') == 'pass' and e.get('Test') == test for e in events)
    summary = dict(test=test, command=command, configuration=dict(court=1, teams=[0, 1], seed=0x12345678, controllerAssignments=[-1,-1,-1,-1] if args.all_computer else [0,-1,-1,-1]), targetUpdates=6000, clockScope='world logical updates; no claim of full match/video clock', exitCode=result.returncode, passed=passed and result.returncode==0, log=log.relative_to(ROOT).as_posix(), logSHA256=hashlib.sha256(result.stdout).hexdigest())
    if failure:
        summary.update(completedUpdates=int(failure[1]), failure=failure[3])
    if before:
        summary['before'] = json.loads(before[1])
    summary['completedRuns'] = [dict(repeat=int(m[1]), updates=int(m[2]), trajectorySHA256=m[3]) for m in re.finditer(r'repeat=(\d+) updates=(\d+).*trajectory_sha256=([0-9a-f]{64})', transcript)]
    (output / (flavor+'-result.json')).write_text(json.dumps(summary, indent=2)+'\n')
    print(json.dumps({k:v for k,v in summary.items() if k!='before'}, indent=2))
    return result.returncode if result.returncode else (0 if passed else 1)


if __name__ == '__main__':
    raise SystemExit(main())
