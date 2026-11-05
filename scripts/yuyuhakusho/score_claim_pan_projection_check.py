"""Source-bound test before correcting the oracle's period/elapsed projection."""
from pathlib import Path
import argparse
import copy
import gzip
import itertools
import json

from score_claim_check import ROOT, load, sha


def native_frames(case):
    index = load(ROOT / case['input_index']['path'])
    if case['id'].startswith('isolated-'):
        code = int(case['id'][9:], 16)
        origin = next(row for row in index['cases'] if row['code'] == code)
        binding = origin['files']['frames']
        path = ROOT / binding['path']
        assert sha(path) == binding['sha256']
        with gzip.open(path, 'rt') as stream:
            for line in stream:
                yield bytes.fromhex(json.loads(line)['driver_hex'])
    else:
        origin = next(row for row in index['cases'] if row['id'] == case['id'])
        binding = origin['files']['original']
        path = ROOT / binding['path']
        assert sha(path) == binding['sha256']
        for frame in load(path)['frames']:
            yield bytes.fromhex(frame['driver_hex'])


def check(index, out):
    report = load(index)
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    for at, raw in ((0x37abe, '1b6d00230024'), (0x37ac8, '102d0024'),
                    (0x37acc, 'b02d0023'), (0x37b18, '522d0024')):
        assert rom[at:at + len(raw) // 2].hex() == raw
    listing = ROOT / '.local/md-yuyuhakusho/audio-score-parameters/parameter-instructions.txt'
    text = listing.read_text()
    for instruction in ('00037abe move.b (0x23,A5),(0x24,A5)',
                        '00037acc cmp.b (0x23,A5),D0b',
                        '00037b18 addq.b #0x1,(0x24,A5)'):
        assert instruction in text
    total = enabled = differences = 0
    cases = sorted(report['cases'], key=lambda case: case['id'] != 'isolated-8a')
    for case in cases:
        path = ROOT / case['file']['path']
        assert sha(path) == case['file']['sha256']
        previous_path = ROOT / case['previous_file']['path'] if 'previous_file' in case else None
        previous = gzip.open(previous_path, 'rt') if previous_path else None
        with gzip.open(path, 'rt') as stream:
            first = json.loads(next(stream))
            if previous:
                old = json.loads(next(previous))
                for voice in old['state']['voices']:
                    pan = voice['pan_automation']
                    pan['period'], pan['elapsed'] = pan['elapsed'], pan['period']
                assert first == old
            for step, (line, raw_driver) in enumerate(itertools.zip_longest(stream, native_frames(case))):
                assert line is not None and raw_driver is not None
                packet = json.loads(line)
                assert packet['step'] == step
                for slot, voice in enumerate(packet['state']['voices']):
                    row = raw_driver[64 + 48 * slot:112 + 48 * slot]
                    active_pan = row[31] != 0 and '.psg' not in voice['slot']
                    expected_period = row[35] if active_pan else 0
                    expected_elapsed = row[36] if active_pan else 0
                    pan = voice['pan_automation']
                    assert (pan['period'], pan['elapsed']) == (expected_period, expected_elapsed), (
                        case['id'], step, voice['slot'], 'period/elapsed',
                        (pan['period'], pan['elapsed']), (expected_period, expected_elapsed))
                    enabled += active_pan
                if previous:
                    old = json.loads(next(previous))
                    for voice in old['state']['voices']:
                        pan = voice['pan_automation']
                        differences += pan['period'] != pan['elapsed']
                        pan['period'], pan['elapsed'] = pan['elapsed'], pan['period']
                    assert packet == old, (case['id'], step, 'Unrelated canonical field changed')
                total += 1
        if previous:
            assert previous.read() == ''
            previous.close()
    assert len(report['cases']) == 40 and total == 125821
    assert report['projection_version'] == 2 and report['exclusions_frozen']
    assert enabled > 0 and differences > 0
    result = {'pass': True, 'cases': 40, 'frames': total,
              'enabled_pan_voice_frames': enabled, 'swapped_voice_frames': differences,
              'only_period_elapsed_changed': True, 'exclusions_unchanged': True,
              'index_sha256': sha(index), 'checker_sha256': sha(Path(__file__)),
              'instruction_listing_sha256': sha(listing),
              'scope': 'Corrected source projection only; independent driver and DSP equality remain separate.'}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.index.resolve(), args.out.resolve())
