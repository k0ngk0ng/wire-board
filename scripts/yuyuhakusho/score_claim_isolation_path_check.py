"""Bound direct PSG restore non-reachability in unchanged isolated controls."""
from pathlib import Path
import argparse
import gzip
import json
from score_claim_check import ROOT, load, sha


def check(index, out):
    report = load(index)
    assert report['pass'] and len(report['cases']) == 23
    listing = ROOT / 'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt'
    text = listing.read_text()
    for instruction in ('0003881a tst.b (A5)', '0003881c bpl.w 0x000388a2',
                        '000388b0 tst.b (A5)', '000388b2 bpl.b 0x000388de',
                        '0003887e beq.b 0x0003888a',
                        '0003917e cmpi.b #-0x20,(0x1,A0)',
                        '00039184 bne.b 0x0003918e',
                        '00039186 move.b (0x25,A0),(0x00c00011).l'):
        assert instruction in text
    counts = {'music_frames_primary_secondary_inactive': 0,
              'effect_frames_music_uninitialized': 0, 'effects_with_no_music_header': 0}
    for case in report['cases']:
        music = case['code'] < 160
        binding = case['files']['frames']
        path = ROOT / binding['path']
        assert sha(path) == binding['sha256']
        with gzip.open(path, 'rt') as stream:
            for line in stream:
                driver = bytes.fromhex(json.loads(line)['driver_hex'])
                assert all(not driver[64 + slot * 48] & 128 for slot in (16, 17))
                if music:
                    assert all(not driver[64 + slot * 48] & 128 for slot in range(10, 16))
                    counts['music_frames_primary_secondary_inactive'] += 1
                else:
                    assert all(not driver[64 + slot * 48] & 128 for slot in range(10))
                    assert driver[64 + 9 * 48 + 1] == 0
                    counts['effect_frames_music_uninitialized'] += 1
        events = load(ROOT / case['files']['events']['path'])['events']
        assert [event['code'] for event in events if event['kind'] == 'header'] == [case['code']]
        if not music:
            assert all(event['pc'] not in (0x384fc, 0x38560, 0x385a0) for event in events)
            counts['effects_with_no_music_header'] += 1
    assert sum(value for key, value in counts.items() if key.endswith('inactive') or key.endswith('uninitialized')) == 122810
    assert counts['effects_with_no_music_header'] == 12
    result = {'pass': True, 'scope': 'Bounded23 isolated inputs and direct-source branch predicates, not global non-reachability',
              'counts': counts, 'index_sha256': sha(index), 'checker_sha256': sha(Path(__file__)),
              'listing_sha256': sha(listing),
              'reasoning': ['Music definitions never activate primary/secondary, so all three restore helpers have inactive effect inputs.',
                            'Effect definitions never initialize music; music PSG3 channel stays0 and cannot satisfy3917e noise-channel==0xe0.',
                            'No source music header executes within effect controls; only music initialization or active music-owned F3 could assign that channel.',
                            'Group2 never starts in these controlled inputs.'],
              'limits': ['Additional mixed inputs require the63-point observer; this does not infer absence from missing60-point traces.']}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.index.resolve(), args.out.resolve())
