"""Reproduce frozen native, rollback, race and Wasm object preparation checks."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / 'games/yuyuhakusho/verification/object-prepare-work'
TEST = ROOT / 'games/yuyuhakusho/engine/object_prepare_work_test.go'
PRODUCTION = ROOT / 'games/yuyuhakusho/engine/object_prepare_work.go'
PACKAGE = './games/yuyuhakusho/engine'
PATTERN = '^TestObjectPrepareWork'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')


def environment():
    return dict(os.environ, TEST_TELEMETRY_DIR=str(ROOT / '.local/go-telemetry'),
                GOCACHE=str(ROOT / '.local/go-cache'), GOMODCACHE=str(ROOT / '.local/go-mod'),
                GOTMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'), TMPDIR=str(ROOT / '.local/md-yuyuhakusho/tmp'),
                GOMAXPROCS='2')


def run(mode, out):
    env = environment()
    go = shutil.which('go')
    assert go
    command = [go, 'test', '-json', '-count=1', '-timeout=5m', '-run', PATTERN]
    if mode == 'race':
        command.append('-race')
    elif mode == 'wasm':
        goroot = subprocess.check_output([go, 'env', 'GOROOT'], cwd=ROOT, env=env, text=True).strip()
        command += ['-exec', str(Path(goroot) / 'lib/wasm/go_js_wasm_exec')]
        env.update(GOOS='js', GOARCH='wasm', GOMAXPROCS='1')
    command.append(PACKAGE)
    if mode == 'vet':
        command = [go, 'vet', PACKAGE]
    start = time.time()
    with (out / 'output.jsonl').open('w') as output:
        result = subprocess.run(command, cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT)
    keys = ['TEST_TELEMETRY_DIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'TMPDIR', 'GOMAXPROCS', 'GOOS', 'GOARCH']
    record = {'mode': mode, 'command': command, 'cwd': str(ROOT), 'environment': {key: env[key] for key in keys if key in env},
              'exit_code': result.returncode, 'elapsed_seconds': time.time() - start, 'output_sha256': sha(out / 'output.jsonl')}
    if mode != 'vet' and result.returncode == 0:
        rows = [json.loads(line) for line in (out / 'output.jsonl').read_text().splitlines()]
        record.update(tests_passed=sum(row['Action'] == 'pass' and 'Test' in row for row in rows),
                      top_level_passed=sum(row['Action'] == 'pass' and 'Test' in row and '/' not in row['Test'] for row in rows),
                      failures=[row for row in rows if row['Action'] in ['fail', 'build-fail', 'skip']])
    write_json(out / 'command.json', record)
    return record


def frozen():
    record = json.loads((BASE / 'test-first.json').read_text())
    assert record['test_first'] and not record['production_exists']
    for path, digest in record['files'].items():
        assert sha(ROOT / path) == digest, path
    red = (BASE / 'red/output.jsonl').read_text()
    assert 'undefined: ObjectDrawState' in red and 'undefined: PrepareObjectDrawWork' in red


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--freeze', type=Path, help='existing two-core native capture directory')
    parser.add_argument('--mode', choices=['native', 'race', 'wasm', 'vet'])
    parser.add_argument('--out', type=Path)
    parser.add_argument('--summarize', action='store_true')
    parser.add_argument('--audit', action='store_true')
    args = parser.parse_args()
    if args.freeze:
        capture = args.freeze.resolve()
        assert capture.is_relative_to(ROOT / '.local') and not PRODUCTION.exists() and not (BASE / 'test-first.json').exists()
        capture_report = json.loads((capture / 'capture.json').read_text())
        assert capture_report['pass'] and capture_report['cases'] == 320 and capture_report['steps'] == 117120
        assert sha(capture / 'native-object-prepare-work.json.gz') == capture_report['fixture_sha256']
        assert sha(capture / 'natural-calls.json.gz') == capture_report['natural_sha256']
        for path, digest in capture_report['sources'].items():
            assert sha(ROOT / path) == digest, path
        for name in ['capture.json', 'native-object-prepare-work.json.gz', 'natural-calls.json.gz']:
            shutil.copyfile(capture / name, BASE / name)
        red = BASE / 'red'
        red.mkdir()
        result = run('native', red)
        output = (red / 'output.jsonl').read_text()
        assert result['exit_code'] != 0 and 'undefined: ObjectDrawState' in output and 'undefined: PrepareObjectDrawWork' in output
        paths = [TEST, Path(__file__), ROOT / 'scripts/yuyuhakusho/capture_object_prepare_work.py', BASE / 'FAILURES.md',
                 BASE / 'capture.json', BASE / 'native-object-prepare-work.json.gz', BASE / 'natural-calls.json.gz']
        record = {'test_first': True, 'production_exists': PRODUCTION.exists(), 'original_capture_pass': True,
                  'actual_missing_api_red': True, 'files': {str(path.relative_to(ROOT)): sha(path) for path in paths}}
        write_json(BASE / 'test-first.json', record)
        print(json.dumps(record))
        return
    frozen()
    if args.audit:
        report = json.loads((BASE / 'validation.json').read_text())
        assert report['pass']
        for group in ['sources', 'artifacts']:
            for path, digest in report[group].items():
                assert sha(ROOT / path) == digest, path
        print(json.dumps({'pass': True, 'sources': len(report['sources']), 'artifacts': len(report['artifacts'])}))
        return
    if args.summarize:
        checks = {mode: json.loads((BASE / mode / 'command.json').read_text()) for mode in ['native', 'race', 'wasm', 'vet']}
        assert all(check['exit_code'] == 0 and not check.get('failures') for check in checks.values())
        assert all(check['top_level_passed'] == 5 and check['tests_passed'] == 85 for mode, check in checks.items() if mode != 'vet')
        source_paths = [PRODUCTION, TEST, ROOT / 'games/yuyuhakusho/engine/main_work.go', Path(__file__), ROOT / 'scripts/yuyuhakusho/capture_object_prepare_work.py']
        artifacts = sorted(path for path in BASE.rglob('*') if path.is_file() and path.name != 'validation.json')
        report = {'pass': True, 'test_first': True, 'scope': 'Independent complete 125A preparation and MainWorkOperation costs; caller/IRQ/audio composition is owned by the joint main clock.',
                  'native_calls': 320, 'native_work_boundaries': 117120, 'natural_calls': 2087, 'natural_reserved_counts': [1, 2],
                  'source_target_sha256': 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06', 'checks': checks,
                  'sources': {str(path.relative_to(ROOT)): sha(path) for path in source_paths},
                  'artifacts': {str(path.relative_to(ROOT)): sha(path) for path in artifacts}}
        write_json(BASE / 'validation.json', report)
        print(json.dumps({'pass': True, 'native_calls': 320, 'native_work_boundaries': 117120, 'checks': list(checks)}))
        return
    assert args.mode and args.out
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and not out.exists()
    out.mkdir(parents=True)
    result = run(args.mode, out)
    print(json.dumps(result))
    raise SystemExit(result['exit_code'])


if __name__ == '__main__':
    main()
