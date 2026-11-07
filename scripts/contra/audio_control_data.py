"""Derive ROM-free per-slot control data from retained original execution.

No guessed lifetimes, media lengths, APU, synthesis or CPU data are embedded.
"""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / 'docs/research/nes-contra/evidence/audio-control-library.json'


def main():
    library = json.loads(SOURCE.read_text())
    assert library['coverage']['complete'] and len(library['definitions']) == 129
    media = json.loads((ROOT / '.local/contra-audio/export/manifest.json').read_text())
    kinds = {s['id']: s['kind'] for s in media['sounds']}
    definitions, targets = [], []
    for source in library['definitions']:
        sid = source['id']
        if sid < 127:
            targets.append({'id': sid, 'slots': [s['slot'] for s in source['entries']]})
        definition = {'id': sid, 'kind': kinds.get(sid, 'control'), 'slots': []}
        for observed in source['slots']:
            if observed['status'] == 'direct-dpcm':
                continue
            assert observed['status'] in ('released', 'loop')
            slot = {'slot': observed['slot'], 'release': observed['releaseTick'],
                    'prefix': [{'tick': r['tick'], 'id': r['id']} for r in observed['prefixRequests']],
                    'loop': [{'tick': r['tick'], 'id': r['id']} for r in observed['loopRequests']],
                    'loopStart': observed['loopStartTick'] or 0, 'loopTicks': observed['loopTicks'] or 0,
                    'end': [{'id': r['id']} for r in observed['endCallbacks']]}
            for variant in library['contextVariants']:
                if variant['id'] == sid and variant['slot'] == observed['slot']:
                    assert variant['releaseTick'] == slot['release']
                    assert variant['context0194'] == 1
                    for request in variant['endCallbacks']:
                        if request not in observed['endCallbacks']:
                            slot['end'].append({'id': request['id'], 'requiresEndPlayersAlive': True})
            definition['slots'].append(slot)
        definitions.append(definition)
    assert len(targets) == 126
    target = ROOT / 'games/contra/engine/audio_control_data.json'
    packed = (json.dumps({'version': 2, 'romSHA256': library['romSHA256'], 'targets': targets,
                         'definitions': definitions}, separators=(',', ':')) + '\n').encode()
    target.write_bytes(packed)
    provenance = {'source_is_original_execution': True, 'romSHA256': library['romSHA256'],
                  'librarySHA256': hashlib.sha256(SOURCE.read_bytes()).hexdigest(),
                  'outputSHA256': hashlib.sha256(packed).hexdigest(), 'definitionCount': len(definitions),
                  'scope': 'Original per-slot lifetimes, internal requests and conditional end callbacks; no audio synthesis or media readiness claim.'}
    target.with_name('audio_control_data-source.json').write_text(json.dumps(provenance, indent=2) + '\n')
    print(json.dumps(provenance))


if __name__ == '__main__':
    main()
