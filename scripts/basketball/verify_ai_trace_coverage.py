"""Check that the recovered-mode fixtures partition the retained original AI trace.

This verifies corpus accounting, not unobserved strategies or a playable match.
Run from the repository root after the fixture extractors and Go tests.
"""
import collections
import csv
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TRACE = ROOT / '.local/nes-basket/runs/ai-shot-observation'
FIXTURES = ROOT / 'games/basketball/engine/testdata'
STEMS = (
    'ai-control', 'ai-possession-control', 'ai-positioning',
    'ai-dribble-attack', 'ai-marking', 'ai-shot-plan',
    'ai-jump-setup', 'ai-engagement', 'ai-dribble-evasion',
)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    with (TRACE / 'ai.csv').open() as stream:
        original = list(csv.DictReader(stream))
    expected = {(int(r['frame']), int(r['actor'])): int(r['mode_before']) & 127 for r in original}
    assert len(expected) == len(original), 'original trace has repeated keys; preserve sequence IDs before comparing'
    raw = (TRACE / 'ai-ram.bin').read_bytes()
    assert len(raw) == len(original) * 4096
    assert (TRACE / 'ram.bin').read_bytes() == (TRACE.parent / 'shot-observation/ram.bin').read_bytes()
    observed = {}
    inventory = []
    for stem in STEMS:
        path = FIXTURES / f'{stem}.json'
        rows = json.loads(path.read_text())
        for row in rows:
            key = (row['frame'], row['actor'])
            assert key not in observed, f'duplicate original call in {stem}: {key}'
            observed[key] = row['before']['mode'] & 127
        inventory.append({'fixture': str(path.relative_to(ROOT)), 'calls': len(rows), 'sha256': digest(path)})
    assert observed == expected, {'missing': sorted(expected.keys() - observed.keys()), 'extra': sorted(observed.keys() - expected.keys())}
    report = {
        'scope': 'all calls in one retained original AI trace; not full-game acceptance',
        'original_calls': len(original), 'fixture_calls': len(observed),
        'duplicate_calls': 0, 'missing_calls': 0,
        'mode_counts': dict(sorted(collections.Counter(expected.values()).items())),
        'trace_sha256': digest(TRACE / 'ai-ram.bin'),
        'trace_csv_sha256': digest(TRACE / 'ai.csv'),
        'ram_matches_uninstrumented': True, 'fixtures': inventory,
    }
    output = ROOT / '.local/basketball/engine/ai-trace-coverage.json'
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({k: v for k, v in report.items() if k != 'fixtures'}, ensure_ascii=False))


if __name__ == '__main__':
    main()
