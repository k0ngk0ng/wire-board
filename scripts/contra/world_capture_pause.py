"""Controller-only whole-world pause/resume evidence before integration.

Failure inventory: entering pause also runs physics; paused frames freeze CHR,
palette or alternating bullet presentation; resuming omits the paused-prefix
presentation before ordinary play; frame counter stops; sound 74 repeats; saves
capture an in-flight campaign phase. No gameplay-memory intervention is used.
"""
import gzip, hashlib, json, os, shutil, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
os.environ['CONTRA_REFERENCE_ROOT'] = str(ROOT / '.local/contra-world/paused-oracle')
from world_capture_complete import HOOK
from capture_reference import BASE, run

def main():
    label = sys.argv[1] if len(sys.argv) > 1 else 'v1'
    (BASE / 'states').mkdir(parents=True, exist_ok=True)
    src = ROOT / '.local/contra/states/coop-play.fcs'
    dst = BASE / 'states/coop-play.fcs'
    if dst.exists():
        assert dst.read_bytes() == src.read_bytes()
    else:
        shutil.copyfile(src, dst)
    spec = {'name': f'paused-world-{label}', 'load_state': 'coop-play', 'frames': 180,
            'captures': [19, 20, 79, 80, 119, 120, 159, 160, 180],
            'segments': [{'start': 1, 'end': 180, 'player': p, 'keys': ['right', 'B']} for p in (1, 2)] +
                        [{'start': n, 'end': n, 'player': 1, 'keys': ['start']} for n in (20, 80, 120, 160)],
            'hooks_lua': HOOK, 'timeout_seconds': 90}
    path = BASE / (spec['name'] + '.json')
    path.write_text(json.dumps(spec, indent=2) + '\n')
    output = run(path)
    rows = [json.loads(line) for line in (output / 'world-frames.jsonl').read_text().splitlines()]
    states = (output / 'world-frames.bin').read_bytes()
    assert len(rows) == 180 and len(states) == len(rows) * 4096
    for i, row in enumerate(rows):
        row['before'] = states[i*4096:i*4096+2048].hex()
        row['after'] = states[i*4096+2048:(i+1)*4096].hex()
    data = {'source_is_original_execution': True, 'scope': 'Controller-only pause/resume whole-world trace; no campaign completion claim.',
            'source': json.loads((output / 'source.json').read_text()), 'scenario': spec,
            'inputTraceSHA256': hashlib.sha256((output / 'inputs.csv').read_bytes()).hexdigest(),
            'ramSHA256': hashlib.sha256((output / 'ram.bin').read_bytes()).hexdigest(), 'frames': rows}
    target = ROOT / 'docs/research/nes-contra/evidence/world-paused-original.json.gz'
    repeated = target.exists()
    if repeated:
        old = json.loads(gzip.decompress(target.read_bytes()))
        assert old['frames'] == rows and old['ramSHA256'] == data['ramSHA256'] and old['inputTraceSHA256'] == data['inputTraceSHA256']
    else:
        target.write_bytes(gzip.compress(json.dumps(data, separators=(',', ':')).encode(), mtime=0))
    print(json.dumps({'frames': len(rows), 'repeatedIdentically': repeated, 'sha256': hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__ == '__main__':
    main()
