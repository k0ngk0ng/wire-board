"""Retain the natural first-period video-runtime gate and its first failure."""
import argparse, hashlib, json, os, re, subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TEST = 'TestRuntimeColdFirstPeriodAndRestore'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group()
    group.add_argument('--race', action='store_true')
    group.add_argument('--wasm', action='store_true')
    parser.add_argument('--full-match', action='store_true', help='require both periods and confirmed original result presentation')
    parser.add_argument('--all-computer', action='store_true', help='independent four-computer full-match gate')
    parser.add_argument('--output')
    args = parser.parse_args()
    full_match = args.full_match or args.all_computer
    default_output = '.local/basketball/video-runtime-all-computer' if args.all_computer else '.local/basketball/video-runtime'
    output = (ROOT / (args.output or default_output)).resolve()
    if not output.is_relative_to(ROOT):
        parser.error('output must stay in the workspace')
    output.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    for key, directory in [('GOCACHE', '.local/go-build'), ('GOMODCACHE', '.local/go-mod'), ('GOTMPDIR', '.local/tmp')]:
        env[key] = str(ROOT / directory)
        Path(env[key]).mkdir(parents=True, exist_ok=True)
    env['TMPDIR'], env['GOTOOLCHAIN'] = env['GOTMPDIR'], 'local'
    test = 'TestRuntimeAllComputerCompleteMatchAndRestore' if args.all_computer else ('TestRuntimeColdCompleteMatchAndRestore' if full_match else TEST)
    command = ['go', 'test', '-json', './games/basketball/engine', '-run', '^'+test+'$', '-count=1']
    flavor = 'race' if args.race else 'wasm' if args.wasm else 'native'
    if args.all_computer: flavor += '-all-computer'
    if full_match: flavor += '-full-match'
    if args.race:
        command.append('-race')
    if args.wasm:
        goroot = subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip()
        command.extend(['-exec', str(Path(goroot)/'lib/wasm/go_js_wasm_exec')])
        env.update(GOOS='js', GOARCH='wasm')
    result = subprocess.run(command,cwd=ROOT,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
    log = output/(flavor+'-tests.jsonl')
    log.write_bytes(result.stdout)
    events = []
    for line in result.stdout.decode(errors='replace').splitlines():
        try: events.append(json.loads(line))
        except json.JSONDecodeError: pass
    transcript = ''.join(e.get('Output','') for e in events)
    passed = result.returncode == 0 and any(e.get('Action')=='pass' and e.get('Test')==test for e in events)
    scope = 'cold configuration, both natural periods and confirmed result presentation, periodic JSON restore; not an original-ROM oracle or campaign/menu proof' if full_match else 'cold configuration, natural first period and next opening, periodic JSON restore; not a full match or original-ROM oracle'
    report = dict(test=test, command=command, passed=passed, exitCode=result.returncode,
        scope=scope, seed=0x12345678, controllerAssignments=[-1,-1,-1,-1] if args.all_computer else [0,-1,-1,-1], maximumVideoFrames=48000 if full_match else 24000, log=str(log.relative_to(ROOT)),
        logSHA256=hashlib.sha256(result.stdout).hexdigest())
    failure = re.search(r'runtime cold seed 12345678 repeat (\d+) video (\d+)/(\d+) logic (\d+): ([^\n]+)',transcript)
    if failure:
        report.update(repeat=int(failure[1]),completedVideoFrames=int(failure[2]),completedLogicSteps=int(failure[4]),failure=failure[5])
    before = re.search(r'^\s*before=(.+)$',transcript,re.MULTILINE)
    if before: report['before'] = json.loads(before[1])
    report['completedRuns'] = [dict(repeat=int(m[1]),videos=int(m[2]),logic=int(m[3]),restores=int(m[4]),trajectorySHA256=m[5])
        for m in re.finditer(r'runtime cold repeat=(\d+) videos=(\d+) logic=(\d+) restores=(\d+).*trajectory_sha256=([0-9a-f]{64})',transcript)]
    (output/(flavor+'-result.json')).write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:v for k,v in report.items() if k!='before'},indent=2))
    return result.returncode if result.returncode else 0 if passed else 1


if __name__ == '__main__': raise SystemExit(main())
