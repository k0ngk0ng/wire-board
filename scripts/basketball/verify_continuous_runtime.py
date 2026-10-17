"""Run the continuous world executable natively and in Wasm, retaining evidence."""
import copy
import gzip
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / '.local/basketball/engine/continuous-runtime'
FIXTURE = ROOT / 'games/basketball/engine/testdata/world-loop.json.gz'


def command(name, args, env=None, succeeds=True):
    result = subprocess.run(args, cwd=ROOT, env=env, capture_output=True, text=True)
    (OUT / (name + '.stdout')).write_text(result.stdout)
    (OUT / (name + '.stderr')).write_text(result.stderr)
    if (result.returncode == 0) != succeeds:
        raise RuntimeError(f'{name}: unexpected exit {result.returncode}; see {OUT}/{name}.stderr')
    return result


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    native = OUT / 'verify-world'
    wasm = OUT / 'verify-world.wasm'
    command('build-native', ['go', 'build', '-o', str(native), './games/basketball/cmd/verify-world'])
    env = dict(os.environ, GOOS='js', GOARCH='wasm')
    command('build-wasm', ['go', 'build', '-o', str(wasm), './games/basketball/cmd/verify-world'], env=env)
    goroot = command('goroot', ['go', 'env', 'GOROOT']).stdout.strip()
    browser_runtime = ['node', str(Path(goroot) / 'lib/wasm/wasm_exec_node.js'), str(wasm)]
    native_result = json.loads(command('native', [str(native)]).stdout)
    wasm_result = json.loads(command('wasm', browser_runtime).stdout)
    assert native_result == wasm_result, 'native and Wasm continuous output differs'
    assert native_result['updates'] == 1998 and native_result['action_calls'] == 26093
    assert native_result['status'] == 'passed' and native_result['state_initializations'] == 1

    rider_fixture = ROOT / 'games/basketball/engine/testdata/world-loop-rider.json.gz'
    rider_native = json.loads(command('rider-native', [str(native), '-fixture', str(rider_fixture)]).stdout)
    rider_wasm = json.loads(command('rider-wasm', browser_runtime + ['-fixture', str(rider_fixture)]).stdout)
    assert rider_native == rider_wasm and rider_native['updates'] == 57
    assert rider_native['state_initializations'] == 1

    fixture = json.loads(gzip.decompress(FIXTURE.read_bytes()))
    cases = []
    for name in ('empty', 'oracle-divergence', 'unsupported-config'):
        broken = copy.deepcopy(fixture)
        if name == 'empty':
            broken['records'] = []
        elif name == 'oracle-divergence':
            broken['records'][-1]['expected']['preparation']['actions']['random'] ^= 1
        else:
            broken['config']['actions']['courtID'] = 255
        path = OUT / (name + '.json.gz')
        path.write_bytes(gzip.compress(json.dumps(broken).encode(), mtime=0))
        for runtime, prefix in [('native', [str(native)]), ('wasm', browser_runtime)]:
            result = command(name + '-' + runtime, prefix + ['-fixture', str(path)], succeeds=False)
            assert result.stderr.strip(), 'failure must explain the rejected replay'
            cases.append(dict(case=name, runtime=runtime, exit=result.returncode))
    sources = sorted((ROOT / 'games/basketball/engine').glob('*.go')) + sorted((ROOT / 'games/basketball/cmd/verify-world').glob('*.go'))
    report = dict(status='passed', scope='single-initial-state core world; no video clock or browser-game acceptance',
                  original_fixture_sha256=hashlib.sha256(FIXTURE.read_bytes()).hexdigest(),
                  source_sha256={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sources},
                  native_and_wasm_identical=True, result=native_result, active_rider_result=rider_native, negative_cases=cases)
    (OUT / 'verification.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(dict(status=report['status'], updates=native_result['updates'], negative_cases=len(cases), artifact=str(OUT/'verification.json'))))


if __name__ == '__main__':
    main()
