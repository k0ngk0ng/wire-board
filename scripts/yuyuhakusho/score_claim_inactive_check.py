"""Test-first proof of StopPrimary restoring an uninitialized music voice."""
from pathlib import Path
import argparse
import json
from score_claim_check import ROOT, clean, load, sha


def check(index, fallback, out):
    report = load(index)
    assert report['pass'] and len(report['cases']) == 1
    case = report['cases'][0]
    assert case['id'] == 'primary-before-first-music' and case['steps'] == 160
    assert case['original_equal'] and case['pcm_samples_equal']
    packet = load(ROOT / case['files']['portable']['path'])
    assert packet['commands'] == [{'step': 0, 'code': 164}, {'step': 10, 'code': 130}]
    assert packet['headers'] == [164, 130]
    provenance = load(ROOT / case['files']['provenance']['path'])['events']
    found = [(i, event) for i, event in enumerate(provenance)
             if event['pc'] == 0x38f78 and event['step'] == 10 and event['row'] == 0xf930]
    assert len(found) == 1
    index_in_trace, event = found[0]
    row = bytes.fromhex(event['row_hex'])
    assert not row[0] & 128 and row[0] & 2 and not row[0] & 4
    assert row[1] == 0 and row[11] == 0
    assert event['read_address'] == event['bank_address'] == 0 and event['instrument_index'] == 0
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assert bytes.fromhex(event['read_hex']) == rom[:25]
    requests = []
    for following in provenance[index_in_trace + 1:]:
        if following['step'] != 10:
            break
        if following['kind'] == 'ym-request':
            requests.append(following)
            if len(requests) == 26:
                break
    registers = [0xb0, 0x30, 0x38, 0x34, 0x3c, 0x50, 0x58, 0x54, 0x5c,
                 0x60, 0x68, 0x64, 0x6c, 0x70, 0x78, 0x74, 0x7c,
                 0x80, 0x88, 0x84, 0x8c, 0x40, 0x48, 0x44, 0x4c, 0xb4]
    assert [item['register'] for item in requests] == registers
    assert all(item['bank'] == 0 for item in requests)
    assert [item['value'] for item in requests[:25]] == list(rom[:25])
    assert requests[-1]['value'] == 0
    data = load(fallback)
    clean(data)
    assert len(data['fm_patches']) == 1 and data['use'] == 'stop-primary-before-first-music'
    patch = data['fm_patches'][0]
    assert patch['algorithm'] == 0 and patch['feedback'] == 0 and len(patch['operators']) == 4
    assert patch['id'].startswith('patch-')
    result = {'pass': True, 'steps': 160, 'fallback_read_step': 10,
              'fallback_patch_id': patch['id'], 'ordered_ym_requests': 26,
              'hardware_bank': 0, 'uninitialized_hardware_channel': 0,
              'index_sha256': sha(index), 'fallback_sha256': sha(fallback),
              'checker_sha256': sha(Path(__file__)),
              'limits': ['Original/observer and typed fallback source proof; Go integration and cadence remain separate.']}
    out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--index', type=Path, required=True)
    parser.add_argument('--fallback', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    check(args.index.resolve(), args.fallback.resolve(), args.out.resolve())
