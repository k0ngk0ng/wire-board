"""Exact test inventory, bounded native/Wasm shards and retained JSON results."""
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
import argparse, hashlib, json, os, re, shutil, subprocess, time

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'games/yuyuhakusho/verification'
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('environment', choices=['native', 'wasm'])
parser.add_argument('--workers', type=int, default=3)
parser.add_argument('--resume', action='store_true')
args = parser.parse_args()
go = shutil.which('go')
node = Path(shutil.which('node')).parent
goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, text=True).strip()
env = {key: os.environ[key] for key in ('HOME', 'LANG', 'LC_ALL', 'LC_CTYPE', 'TZ') if key in os.environ}
env.update(PATH=str(Path(go).parent) + ':' + str(node) + ':/usr/bin:/bin', GOCACHE=str(ROOT / '.local/go-cache'), GOMODCACHE=str(ROOT / '.local/go-mod'), GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), TMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'))
if args.environment == 'wasm':
    env.update(GOOS='js', GOARCH='wasm')
base = [go, 'test']
if args.environment == 'wasm':
    base += ['-exec', goroot + '/lib/wasm/go_js_wasm_exec']
listed = subprocess.run(base + ['-list', '^Test', './games/yuyuhakusho/...'], cwd=ROOT, env=env, text=True, capture_output=True, check=True)
packages = {}
current = []
for line in listed.stdout.splitlines():
    if re.fullmatch(r'Test\w+', line):
        current.append(line)
    elif line.startswith('ok\t') or line.startswith('ok '):
        package = line.split()[1]
        assert package not in packages
        packages[package] = sorted(current)
        current = []
assert not current and any(package.endswith('/engine') for package in packages)

# A trie keeps the exact Wasm regex comfortably below Go's8KiB argv/env limit.
def trie_regex(names):
    trie = {}
    for name in names:
        node = trie
        for character in name:
            node = node.setdefault(character, {})
        node[''] = {}
    def encode(node):
        terminal = '' in node
        choices = [re.escape(key) + encode(child) for key, child in sorted(node.items()) if key]
        text = choices[0] if len(choices) == 1 else '(?:' + '|'.join(choices) + ')' if choices else ''
        if terminal and text:
            text = '(?:' + text + ')?'
        return text
    return '^' + encode(trie) + '$'

jobs = []
for package, tests in packages.items():
    shards = 8 if package.endswith('/engine') else 1
    for index in range(shards):
        selected = tests[index::shards]
        if selected:
            label = package.rsplit('/', 1)[-1] + '-' + str(index)
            jobs.append((label, package, selected))
identity_paths = sorted((ROOT / 'games/yuyuhakusho/engine').glob('*.go')) + sorted((ROOT / 'games/yuyuhakusho/presentation').glob('*.go')) + sorted((ROOT / 'games/yuyuhakusho/runtimebridge').glob('*.go'))
identity = {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest() for path in identity_paths}
plan_path = OUT / ('additional-arena-' + args.environment + '-checks-plan.json')
plan = {'environment': args.environment, 'packages': packages, 'jobs': [{'label': label, 'package': package, 'tests': selected} for label, package, selected in jobs], 'source_sha256': identity}
if args.resume and plan_path.exists():
    assert json.loads(plan_path.read_text()) == plan, 'cannot resume changed test inventory or source'
plan_path.write_text(json.dumps(plan, indent=2) + '\n')

def run(job):
    label, package, selected = job
    path = OUT / ('additional-arena-' + args.environment + '-checks-' + label + '.jsonl')
    result_path = path.with_suffix('.result.json')
    if args.resume and result_path.is_file():
        old = json.loads(result_path.read_text())
        if old['exit_code'] == 0 and old['log_sha256'] == hashlib.sha256(path.read_bytes()).hexdigest():
            return old
    command = base + ['-json', '-count=1', '-timeout=40m']
    if args.environment == 'native':
        command.append('-race')
    command += ['-run', trie_regex(selected), package]
    assert sum(len(key) + len(value) + 2 for key, value in env.items()) + sum(len(arg) + 1 for arg in command) < 7900
    started = time.monotonic()
    with path.open('w') as output:
        result = subprocess.run(command, cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT)
    passed = []
    failed = []
    for line in path.read_text().splitlines():
        event = json.loads(line)
        if event.get('Test') and '/' not in event['Test']:
            if event['Action'] == 'pass':
                passed.append(event['Test'])
            elif event['Action'] == 'fail':
                failed.append(event['Test'])
    report = {'label': label, 'package': package, 'command': command, 'exit_code': result.returncode, 'elapsed_seconds': round(time.monotonic() - started, 3), 'expected': selected, 'passed': sorted(passed), 'failed': sorted(failed), 'log': str(path.relative_to(ROOT)), 'log_sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
    result_path.write_text(json.dumps(report, indent=2) + '\n')
    assert result.returncode == 0 and sorted(passed) == selected and not failed, (label, result.returncode, sorted(set(selected) - set(passed)))
    return report

reports = []
with ThreadPoolExecutor(max_workers=args.workers) as pool:
    for future in as_completed([pool.submit(run, job) for job in jobs]):
        report = future.result()
        reports.append(report)
        print(args.environment, report['label'], len(report['passed']), 'pass', report['elapsed_seconds'], 'seconds', flush=True)
assert sum(len(report['passed']) for report in reports) == sum(map(len, packages.values()))
summary = {'pass': True, 'environment': args.environment, 'tests': sum(len(report['passed']) for report in reports), 'packages': {package: len(tests) for package, tests in packages.items()}, 'jobs': sorted(reports, key=lambda report: report['label']), 'plan': str(plan_path.relative_to(ROOT)), 'source_sha256': identity}
(OUT / ('additional-arena-' + args.environment + '-checks.json')).write_text(json.dumps(summary, indent=2) + '\n')
print(args.environment, summary['tests'], 'exact top-level tests pass', flush=True)
