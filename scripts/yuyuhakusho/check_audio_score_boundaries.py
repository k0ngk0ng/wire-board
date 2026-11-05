"""Frozen acceptance for actual extended PSG reads and DAC end-of-tick policy."""
from pathlib import Path
import argparse
import gzip
import hashlib
import json

ROOT = Path(__file__).resolve().parents[2]
EXTENDED = {121: 3085, 122: 3599, 123: 4113, 124: 4627,
            125: 5248, 126: 55522, 127: 60662}
READ_COUNTS = {121: 24, 122: 28, 123: 49, 124: 25,
               125: 64, 126: 8, 127: 84}


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)


def check(catalog, index, out):
    library, oracle = load(catalog), load(index)
    assert oracle['pass'] and len(library['scores']) == 23
    assert library['pitch_tables']['psg_extended_dividers'] == [
        {'index': key, 'divider': value} for key, value in EXTENDED.items()]
    assert len(library['pitch_tables']['psg_dividers']) == 70
    counts, actual_pitch_reads = dict.fromkeys(EXTENDED, 0), 0
    missing_returns = []
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assert hashlib.sha256(rom).hexdigest() == oracle['rom_sha256']
    for score, case in zip(library['scores'], oracle['cases']):
        assert score['id'] == case['code']
        path = ROOT / case['files']['events']['path']
        assert sha(path) == case['files']['events']['sha256']
        events = load(path)['events']
        before = {e['step']: e for e in events if e['kind'] == 'tick-before'}
        returned = {e['step'] for e in events if e['kind'] == 'tick-return'}
        for step in sorted(set(before) - returned):
            missing_returns.append([score['id'], step])
        for voice in score['voices']:
            timeline = voice['timeline']
            if timeline['cycle']:
                assert sum(e['semitones'] for e in timeline['cycle']
                           if e['kind'] == 'transpose-add') % 256 == 0
            for event in timeline['prefix'] + timeline['cycle']:
                if event['kind'] == 'track-end':
                    assert event['ends_driver_tick'] == (voice['channel'] == 'dac')
        for event in events:
            if event['kind'] != 'parameter-outcome' or event['pc'] != 0x3a072:
                continue
            actual_pitch_reads += 1
            assert event['d5'] % 2 == 0 and 0 <= event['d5'] <= 254
            pitch_index = event['d5'] // 2
            word = int.from_bytes(bytes.fromhex(event['row_hex'])[16:18], 'big')
            address = 0x3a1a6 + pitch_index * 2
            assert word == int.from_bytes(rom[address:address + 2], 'big')
            if pitch_index >= 70:
                assert EXTENDED[pitch_index] == word
                counts[pitch_index] += 1
            else:
                assert library['pitch_tables']['psg_dividers'][pitch_index] == word
    assert counts == READ_COUNTS and actual_pitch_reads == 24873
    assert missing_returns == [[code, 0] for code in oracle['codes'][:11]] + [[143, 165]]
    result = {
        'pass': True, 'dsp_implemented': False,
        'actual_psg_pitch_reads': actual_pitch_reads,
        'actual_extended_psg_pitch_reads': sum(counts.values()),
        'extended_psg_indices': list(EXTENDED),
        'native_tick_exit_boundaries': missing_returns,
        'catalog_sha256': sha(catalog), 'oracle_index_sha256': sha(index),
        'checker_sha256': sha(Path(__file__).resolve()),
        'limits': ['Actual pitch reads and typed track-end policy only; '
                   'continuous note, modulation, bus and DSP proofs are separate.'],
    }
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--catalog', type=Path, required=True)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.catalog.resolve(), args.index.resolve(), args.out.resolve())
