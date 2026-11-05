"""Compare all165 default-arena rule reports and14 rendered source7 matches."""
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
import argparse, gzip, hashlib, json, os, subprocess

ROOT = Path(__file__).resolve().parents[2]
LOCAL = ROOT / '.local/md-yuyuhakusho'
OUT = ROOT / 'games/yuyuhakusho/verification'
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('environment', choices=['native', 'wasm'])
parser.add_argument('--workers', type=int, default=2)
args = parser.parse_args()
env = dict(os.environ, GOCACHE=str(ROOT / '.local/go-cache'), GOMODCACHE=str(ROOT / '.local/go-mod'), GOTMPDIR=str(LOCAL / 'tmp'), TMPDIR=str(LOCAL / 'tmp'), TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'))
if args.environment == 'wasm':
    env.update(GOOS='js', GOARCH='wasm')
commands = {}
for kind, package in [('rules', 'verify-chu-session'), ('render', 'verify-chu-runtime')]:
    binary = LOCAL / 'build' / ('arena-legacy-' + kind + ('.wasm' if args.environment == 'wasm' else ''))
    subprocess.run(['go', 'build', '-o', str(binary), './games/yuyuhakusho/cmd/' + package], cwd=ROOT, env=env, check=True)
    commands[kind] = [str(binary)]
    if args.environment == 'wasm':
        goroot = subprocess.check_output(['go', 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        commands[kind].insert(0, str(Path(goroot) / 'lib/wasm/go_js_wasm_exec'))

paths = [(path, 'rules') for path in sorted((OUT / 'session-e2e' / args.environment).glob('*.json'))]
for stage in ('fighter10-e2e', 'genkai-e2e', 'yusuke-e2e', 'kuwabara-e2e', 'elder-e2e', 'kurama-e2e', 'jin-e2e', 'hiei-e2e', 'sensui-e2e', 'itsuki-e2e'):
    paths += [(path, 'rules') for path in sorted((OUT / stage / args.environment / 'rules').glob('*.json'))]
assert len(paths) == 165, len(paths)
paths += [(path, 'render') for path in sorted((OUT / 'itsuki-e2e' / args.environment / 'render').glob('*.json'))]
assert len(paths) == 179, len(paths)

# Deduplicate identical command configurations while preserving every old report binding.
groups = {}
for path, kind in paths:
    expected = json.loads(path.read_text())
    options = ['-players', str(expected['players'])]
    if expected.get('characters'):
        options += ['-characters', ','.join(map(str, expected['characters']))]
    if expected['team']:
        options += ['-team']
    if expected.get('human_idle'):
        options += ['-human-idle']
    groups.setdefault((kind, tuple(options)), []).append(path)

checks, actuals = [], {}
def run_group(item):
    (kind, options), old_paths = item
    command = commands[kind] + list(options)
    if kind == 'render':
        name = hashlib.sha256(json.dumps(options).encode()).hexdigest()[:12]
        command += ['-output', str(LOCAL / 'arena-legacy-previews' / args.environment / name)]
    result = subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True)
    assert result.returncode == 0, (command, result.stdout, result.stderr)
    actual = json.loads(result.stdout)
    assert actual['pass'] and not actual['reference_state_loaded']
    results = []
    for path in old_paths:
        expected = json.loads(path.read_text())
        a, b = actual.copy(), expected.copy()
        if kind == 'rules':
            a.pop('rules_sha256'); b.pop('rules_sha256')
            excluded = ['rules_sha256']
        else:
            # Rules SHA is deliberately part of the saved asset digests; the
            # old pixel trace and all independent lifecycle checks stay exact.
            excluded = ['final_state_sha256', 'state_trace_sha256']
            for key in excluded:
                a.pop(key); b.pop(key)
        assert a == b, (str(path), {key: (b.get(key), a.get(key)) for key in b.keys() | a.keys() if b.get(key) != a.get(key)})
        key = str(path.relative_to(OUT))
        results.append((key, actual, {'expected_path': str(path.relative_to(ROOT)), 'expected_sha256': hashlib.sha256(path.read_bytes()).hexdigest(), 'kind': kind, 'pass': True, 'excluded_version_bound_fields': excluded}))
    return results

with ThreadPoolExecutor(max_workers=args.workers) as pool:
    futures = [pool.submit(run_group, group) for group in groups.items()]
    for number, future in enumerate(as_completed(futures), 1):
        for key, actual, check in future.result():
            checks.append(check); actuals[key] = actual
        print(args.environment, str(number) + '/' + str(len(groups)), 'configurations,', len(checks), 'historical reports preserve rules/pixels', flush=True)
assert len(checks) == len(actuals) == 179
raw = (json.dumps(actuals, separators=(',', ':')) + '\n').encode()
packed = gzip.compress(raw, mtime=0)
path = OUT / ('additional-arena-legacy-' + args.environment + '-actual.json.gz')
path.write_bytes(packed)
report = {'pass': True, 'environment': args.environment, 'reports': len(checks), 'rule_reports': 165, 'render_reports': 14, 'unique_configurations_executed': len(groups), 'checks': sorted(checks, key=lambda check: check['expected_path']), 'actual_reports_path': str(path.relative_to(ROOT)), 'actual_reports_sha256': hashlib.sha256(raw).hexdigest(), 'actual_reports_gzip_sha256': hashlib.sha256(packed).hexdigest(), 'limits': ['Rule reports preserve complete final world/event/lifecycle traces; only rule-data digest may change. Render reports preserve complete pixel traces and lifecycle checks; snapshot hashes embed the deliberately changed rules asset digest and are compared separately by unchanged legacy JSON fixtures and native/Wasm equality.']}
(OUT / ('additional-arena-legacy-' + args.environment + '.json')).write_text(json.dumps(report, indent=2) + '\n')
print('all179 historical reports pass', args.environment)
