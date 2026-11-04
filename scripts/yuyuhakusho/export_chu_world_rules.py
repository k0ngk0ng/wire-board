#!/usr/bin/env python3
"""Export semantic world rules; never include original CPU snapshots or media."""
import gzip
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def main():
    sources = []

    def read(name):
        source = ROOT / name
        packed = source.read_bytes()
        raw = gzip.decompress(packed) if name.endswith('.gz') else packed
        sources.append({'path': name, 'sha256': sha(packed)})
        value = json.loads(raw)
        if 'rom_sha256' in value:
            assert value['rom_sha256'] == ROM_SHA
        return value

    data = 'games/yuyuhakusho/data/'
    tests = 'games/yuyuhakusho/engine/testdata/'
    fighters = read(data + 'fighter-specs.json')['specs']
    commands = read(data + 'command-programs.json')['library']
    parents = read(data + 'animation-catalog.json.gz')['frames']
    projectiles = read(data + 'chu-projectile-animation.json')['frames']
    attachments = read(tests + 'chu-attachments-oracle.json.gz')
    effects = read(data + 'effect-sequences.json')['library']
    attached = read(tests + 'attached-effects-data.json')
    start = read(tests + 'match-start-library.json')
    arena = read(data + 'arena-runtime-rules.json')['library']
    fighter0_projectiles = read(data + 'fighter0-projectile-rules.json')

    def animations(frames):
        output, seen = [], {}
        for frame in frames:
            identity = json.dumps(frame['key'], sort_keys=True)
            if identity in seen:
                assert seen[identity] == frame['data'], ('conflicting semantic animation', identity)
                continue
            seen[identity] = frame['data']
            output.append({'key': frame['key'], 'data': frame['data']})
        return output

    payload = {'schema': 2, 'rom_sha256': ROM_SHA, 'fighters': fighters, 'commands': commands,
               'animations': animations(parents + projectiles + fighter0_projectiles['animations']),
               'attachments': {'sequences': attachments['sequences'], 'animations': animations(attachments['animations'])},
               'effects': effects,
               'attached_effects': {key: attached[key] for key in ['sequences', 'orbit_x', 'orbit_y', 'offset_y']},
               'match_start': start, 'arena': arena,
               'fighter0_projectiles': {'sequences': fighter0_projectiles['sequences']}}
    raw = (json.dumps(payload, separators=(',', ':')) + '\n').encode()
    packed = gzip.compress(raw, mtime=0)
    target = ROOT / data / 'chu-world-rules.json.gz'
    target.write_bytes(packed)
    manifest = {'schema': 2, 'scope': 'Immutable independently decoded semantic rules only; no calls, RAM images, executable code or client media',
                'rom_sha256': ROM_SHA, 'sources': sources, 'exporter_sha256': sha(Path(__file__).read_bytes()),
                'counts': {'fighters': len(fighters), 'command_programs': len(commands), 'animations': len(payload['animations']),
                           'charge_sequences': len(payload['attachments']['sequences']), 'release_animations': len(payload['attachments']['animations']),
                           'effect_types': len(effects['types']), 'effect_sequences': len(effects['sequences']),
                           'attached_sequences': len(payload['attached_effects']['sequences']),
                           'fighter0_projectile_frames': len(fighter0_projectiles['animations']),
                           'fighter0_release_sequences': len(fighter0_projectiles['sequences'])},
                'bytes': len(raw), 'gzip_bytes': len(packed), 'sha256': sha(raw), 'gzip_sha256': sha(packed)}
    (ROOT / data / 'chu-world-rules-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps({key: value for key, value in manifest.items() if key != 'sources'}, indent=2))


if __name__ == '__main__':
    main()
