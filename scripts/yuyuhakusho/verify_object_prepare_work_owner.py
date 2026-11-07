"""Verify direct caller-owned drawing preparation without changing the native fixture."""
import argparse
import json
from pathlib import Path

import verify_object_prepare_work as original

ROOT, BASE = original.ROOT, original.BASE
TEST = ROOT / 'games/yuyuhakusho/engine/object_prepare_work_owner_test.go'
PRODUCTION = original.PRODUCTION
OWNER_ROOT = BASE / 'owner'
MODES = ['native', 'race', 'wasm', 'vet']
original.PATTERN = '^TestObjectPrepare(Work|Owner)'


def frozen():
    original.frozen()
    record = json.loads((OWNER_ROOT / 'test-first.json').read_text())
    assert record['test_first'] and not record['direct_owner_api_exists']
    for path, digest in record['files'].items():
        assert original.sha(ROOT / path) == digest, path
    red = (OWNER_ROOT / 'red/output.jsonl').read_text()
    assert 'undefined: PrepareObjectDrawOnOwner' in red


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--freeze', action='store_true')
    parser.add_argument('--mode', choices=MODES)
    parser.add_argument('--out', type=Path)
    parser.add_argument('--summarize', action='store_true')
    parser.add_argument('--audit', action='store_true')
    args = parser.parse_args()
    if args.freeze:
        original.frozen()
        assert 'func PrepareObjectDrawOnOwner(' not in PRODUCTION.read_text()
        assert not OWNER_ROOT.exists()
        (OWNER_ROOT / 'red').mkdir(parents=True)
        result = original.run('native', OWNER_ROOT / 'red')
        assert result['exit_code'] != 0
        assert 'undefined: PrepareObjectDrawOnOwner' in (OWNER_ROOT / 'red/output.jsonl').read_text()
        paths = [TEST, Path(__file__), BASE / 'OWNER_FAILURES.md', BASE / 'test-first.json']
        record = {'test_first': True, 'direct_owner_api_exists': False,
                  'production_before_refactor_sha256': original.sha(PRODUCTION),
                  'native_fixture_unchanged': True, 'actual_missing_api_red': True,
                  'files': {str(path.relative_to(ROOT)): original.sha(path) for path in paths}}
        original.write_json(OWNER_ROOT / 'test-first.json', record)
        print(json.dumps(record))
        return
    frozen()
    if args.audit:
        report = json.loads((BASE / 'validation.json').read_text())
        assert report['pass'] and report['direct_owner_api_verified']
        for group in ['sources', 'artifacts']:
            for path, digest in report[group].items():
                assert original.sha(ROOT / path) == digest, path
        print(json.dumps({'pass': True, 'sources': len(report['sources']), 'artifacts': len(report['artifacts'])}))
        return
    if args.summarize:
        checks = {mode: json.loads((OWNER_ROOT / mode / 'command.json').read_text()) for mode in MODES}
        assert all(check['exit_code'] == 0 and not check.get('failures') for check in checks.values())
        assert all(check['top_level_passed'] == 9 and check['tests_passed'] == 89 for mode, check in checks.items() if mode != 'vet')
        previous = json.loads((BASE / 'validation.json').read_text())
        paths = [ROOT / path for path in previous['sources']] + [TEST, Path(__file__)]
        artifacts = sorted(path for path in BASE.rglob('*') if path.is_file() and path.name != 'validation.json')
        previous.update(direct_owner_api_verified=True, native_fixture_unchanged=True,
                        scope='Complete original drawing preparation through caller-owned state and value wrapper; exact native work boundaries, live publication, direct-owner error frontier and value rollback. Joint main clock owns IRQ/audio acceptance.',
                        checks=checks, sources={str(path.relative_to(ROOT)): original.sha(path) for path in paths},
                        artifacts={str(path.relative_to(ROOT)): original.sha(path) for path in artifacts})
        original.write_json(BASE / 'validation.json', previous)
        print(json.dumps({'pass': True, 'native_fixture_unchanged': True, 'tests_per_runtime': 89, 'checks': list(checks)}))
        return
    assert args.mode and args.out
    out = args.out.resolve()
    assert out.is_relative_to(ROOT) and not out.exists()
    out.mkdir(parents=True)
    record = original.run(args.mode, out)
    print(json.dumps(record))
    raise SystemExit(record['exit_code'])


if __name__ == '__main__':
    main()
