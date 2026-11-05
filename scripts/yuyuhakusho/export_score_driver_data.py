"""Official typed driver catalog; prior note-only producer stays frozen."""
from pathlib import Path
import argparse
import gzip
import hashlib
import json

from export_audio_score import build, canonical
from score_parameter_tables import read_fm_patch, ROM_SHA

ROOT = Path(__file__).resolve().parents[2]


def export(out, fallback):
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    assert hashlib.sha256(rom).hexdigest() == ROM_SHA
    library, provenance = build(rom)
    patches = {patch['id']: patch for patch in library['fm_patches']}
    selections = []
    for score in library['scores']:
        bank = provenance[str(score['id'])]['instrument_bank']
        for voice in score['voices']:
            if not voice['channel'].startswith('fm'):
                continue
            for event in voice['timeline']['prefix'] + voice['timeline']['cycle']:
                if event['kind'] != 'volume-envelope':
                    continue
                index = event['envelope_id']
                data = read_fm_patch(rom, bank, index).to_score_data()
                data.pop('id', None)
                identity = 'patch-' + hashlib.sha256(canonical(data)).hexdigest()[:20]
                patches[identity] = {'id': identity, **data}
                selections.append({'score_id': score['id'], 'voice_id': voice['id'],
                                   'at': event['at'], 'bank': bank, 'index': index,
                                   'patch_id': identity})
                event.clear()
                event.update(at=selections[-1]['at'], kind='fm-patch-select', patch_id=identity)
    assert len(selections) == 1 and selections[0]['score_id'] == 0x95 and selections[0]['index'] == 74
    library.update(schema=2, format='typed-audio-score-v2',
                   fm_patches=[patches[key] for key in sorted(patches)])
    provenance['silent_fm_selections'] = selections
    if fallback is not None:
        entries = json.loads(fallback.read_text())
        # A separately source-verified uninitialized restore dependency may be
        # supplied by the claim investigation; never read its ROM location here.
        assert entries['schema'] == 1 and entries['use'] == 'stop-primary-before-first-music'
        assert len(entries['fm_patches']) == 1
        for patch in entries['fm_patches']:
            patches[patch['id']] = patch
        library['fm_patches'] = [patches[key] for key in sorted(patches)]
        library['fallback_patch_id'] = entries['fm_patches'][0]['id']
    out.mkdir(parents=True, exist_ok=True)
    raw = (json.dumps(library, separators=(',', ':')) + '\n').encode()
    (out / 'catalog.json.gz').write_bytes(gzip.compress(raw, mtime=0))
    (out / 'provenance.json').write_text(json.dumps(provenance, indent=2) + '\n')
    print(json.dumps({'schema': 2, 'definitions': len(library['scores']),
                      'patches': len(library['fm_patches']),
                      'silent_fm_selections': len(selections),
                      'sha256': hashlib.sha256((out / 'catalog.json.gz').read_bytes()).hexdigest()}, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--fallback', type=Path, required=True)
    args = parser.parse_args()
    target = args.out.resolve()
    assert target.is_relative_to(ROOT)
    source = args.fallback.resolve()
    assert source.is_relative_to(ROOT)
    export(target, source)
