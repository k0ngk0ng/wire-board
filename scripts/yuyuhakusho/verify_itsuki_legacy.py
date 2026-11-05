"""Re-run all140 prior rule scenarios without overwriting historical reports."""
from pathlib import Path
import gzip, hashlib, json, os, subprocess, sys

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'games/yuyuhakusho/verification'
LOCAL = ROOT / '.local/md-yuyuhakusho'
target = sys.argv[1]
assert target in ('native', 'wasm')
env = dict(os.environ, GOCACHE=str(ROOT / '.local/go-cache'),
           GOMODCACHE=str(ROOT / '.local/go-mod'), GOTMPDIR=str(LOCAL / 'tmp'), TMPDIR=str(LOCAL / 'tmp'))
binary = LOCAL / 'build' / ('itsuki-legacy' + ('.wasm' if target == 'wasm' else ''))
binary.parent.mkdir(parents=True, exist_ok=True)
build_env = dict(env, GOOS='js', GOARCH='wasm') if target == 'wasm' else env
subprocess.run(['go', 'build', '-o', str(binary), './games/yuyuhakusho/cmd/verify-chu-session'],
               cwd=ROOT, env=build_env, check=True)
command = [str(binary)]
if target == 'wasm':
    goroot = subprocess.check_output(['go', 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
    command.insert(0, str(Path(goroot) / 'lib/wasm/go_js_wasm_exec'))

paths = sorted((OUT / 'session-e2e' / target).glob('*.json'))
for stage in ('fighter10-e2e', 'genkai-e2e', 'yusuke-e2e', 'kuwabara-e2e', 'elder-e2e', 'kurama-e2e', 'jin-e2e', 'hiei-e2e', 'sensui-e2e'):
    paths += sorted((OUT / stage / target / 'rules').glob('*.json'))
assert len(paths) == 140, len(paths)
checks, actuals = [], {}
for expected_path in paths:
    expected = json.loads(expected_path.read_text())
    args = ['-players', str(expected['players'])]
    if expected.get('characters'):
        args += ['-characters', ','.join(map(str, expected['characters']))]
    if expected['team']:
        args += ['-team']
    if expected['human_idle']:
        args += ['-human-idle']
    actual = json.loads(subprocess.check_output(command + args, cwd=ROOT, env=env, text=True))
    assert actual['pass'], expected_path
    a, b = actual.copy(), expected.copy()
    current_rules, prior_rules = a.pop('rules_sha256'), b.pop('rules_sha256')
    assert a == b, (str(expected_path), 'prior world/event/lifecycle trace changed')
    key = str(expected_path.relative_to(OUT))
    actuals[key] = actual
    checks.append({'environment': target, 'case': key, 'pass': True,
                   'expected_path': str(expected_path.relative_to(ROOT)),
                   'expected_sha256': hashlib.sha256(expected_path.read_bytes()).hexdigest(),
                   'actual_key': key, 'previous_rules_sha256': prior_rules,
                   'rules_sha256': current_rules, 'full_json_equal': actual == expected})
    print(key, actual['ticks'], 'pass', flush=True)

partial = {'checks': checks, 'actuals': actuals}
(LOCAL / ('itsuki-legacy-' + target + '.json.gz')).write_bytes(
    gzip.compress((json.dumps(partial, separators=(',', ':')) + '\n').encode(), mtime=0))
if target == 'wasm':
    native = json.loads(gzip.decompress((LOCAL / 'itsuki-legacy-native.json.gz').read_bytes()))
    checks = native['checks'] + checks
    actuals.update(native['actuals'])
    assert len(checks) == len(actuals) == 280
    raw = (json.dumps(actuals, separators=(',', ':')) + '\n').encode()
    packed = gzip.compress(raw, mtime=0)
    path = OUT / 'itsuki-legacy-actual.json.gz'
    path.write_bytes(packed)
    report = {'scope': 'All140 prior source0/1/2/3/4/5/6/8/9/10 rule configurations in native/Wasm after source7 support',
              'pass': True, 'checks': checks, 'actual_reports_path': str(path.relative_to(ROOT)),
              'actual_reports_sha256': hashlib.sha256(raw).hexdigest(),
              'actual_reports_gzip_sha256': hashlib.sha256(packed).hexdigest(),
              'limits': ['Only rule-data digests may differ when the historical schema1/schema2 additions are compared; schema3 adds source7 typed animation data while prior world/event/lifecycle traces stay exact; all world/event/lifecycle fields must match.']}
    (OUT / 'itsuki-legacy-regression.json').write_text(json.dumps(report, indent=2) + '\n')
    print('all280 prior rule reports preserve complete world/event/lifecycle traces')
