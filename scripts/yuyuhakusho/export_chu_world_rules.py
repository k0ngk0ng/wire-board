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
    arena_linear = read(data + 'arena-scene-linear-rows.json.gz')
    arena_four = read(data + 'arena-scene-4-tables.json.gz')
    assert len(arena_linear['linear_rows']) == 241
    assert all(len(arena_linear['linear_rows'][str(x)]) == 128 and
               arena_linear['linear_rows'][str(x)][40:96] == arena['tails'][str(x)]
               for x in range(744, 985))
    assert len(arena_four['arena4_rows']) == 64
    assert all(len(arena_four['arena4_rows'][str(x)]) == 112 for x in range(64))
    assert len(arena_four['arena4_colors']) == 4 and all(len(row) == 3 for row in arena_four['arena4_colors'])
    arena_resources = []
    for index, label in enumerate(['default', '2', '4', '6', '8', 'a', 'c', 'e']):
        asset = read(data + f'arena-assets-{label}.json.gz')
        assert asset['resource_binding']['resource_list'] == start['arena_resources'][index]
        palette = asset['initial_scene_palette_0']
        assert len(palette) == 16 and all(0 <= color < 512 for color in palette)
        # Assets expose compact 3-bit RGB. Fade rules own actual CRAM words.
        cram = [((color & 7) << 1) | (((color >> 3) & 7) << 5) | (((color >> 6) & 7) << 9)
                for color in palette]
        arena_resources.append({'arena': index * 2, 'source': start['arena_resources'][index], 'palette_0': cram})
    fighter0_projectiles = read(data + 'fighter0-projectile-rules.json')
    fighter2_projectiles = read(data + 'fighter2-projectile-rules.json')
    fighter1_projectiles = read(data + 'fighter1-projectile-rules.json')
    fighter4_projectiles = read(tests + 'fighter4-projectile-rules.json')
    fighter4_movements = read(tests + 'fighter4-composition-movement-rules.json')
    fighter5_projectiles = read(tests + 'fighter5-projectile-data.json')
    fighter7_children = read(data + 'fighter7-children-rules.json')
    assert len(fighter7_children['frames']) == 94
    assert fighter5_projectiles['source_sha256'] == ROM_SHA and len(fighter5_projectiles['frames']) == 33
    alternate_movements = [{'source': rule['source'], 'movement': rule['movement']}
                           for rule in fighter4_movements['rules'] if rule['source'] == 0x15946]
    assert len(alternate_movements) == 1
    assert len(fighter4_projectiles['animations']) == 9
    assert fighter4_projectiles['curve']['first_index'] == -32
    assert len(fighter4_projectiles['curve']['values']) == 1344

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

    payload = {'schema': 3, 'rom_sha256': ROM_SHA, 'fighters': fighters, 'commands': commands,
               'animations': animations(parents + projectiles + fighter0_projectiles['animations'] + fighter2_projectiles['animations'] + fighter1_projectiles['animations'] + fighter4_projectiles['animations'] + fighter5_projectiles['frames'] + fighter7_children['frames']),
               'attachments': {'sequences': attachments['sequences'], 'animations': animations(attachments['animations'])},
               'effects': effects,
               'attached_effects': {key: attached[key] for key in ['sequences', 'orbit_x', 'orbit_y', 'offset_y']},
               'match_start': start, 'arena': arena,
               'arena_scenes': {'linear_rows': arena_linear['linear_rows'],
                                'arena4_rows': arena_four['arena4_rows'],
                                'arena4_colors': arena_four['arena4_colors']},
               'arena_resources': arena_resources,
               'fighter0_projectiles': {'sequences': fighter0_projectiles['sequences']},
               'alternate_movements': alternate_movements,
               'fighter4_dragon': {'curve': fighter4_projectiles['curve']}}
    raw = (json.dumps(payload, separators=(',', ':')) + '\n').encode()
    packed = gzip.compress(raw, mtime=0)
    target = ROOT / data / 'chu-world-rules.json.gz'
    target.write_bytes(packed)
    manifest = {'schema': 3, 'scope': 'Immutable independently decoded semantic rules only; no calls, RAM images, executable code or client media',
                'rom_sha256': ROM_SHA, 'sources': sources, 'exporter_sha256': sha(Path(__file__).read_bytes()),
                'counts': {'fighters': len(fighters), 'command_programs': len(commands), 'animations': len(payload['animations']),
                           'charge_sequences': len(payload['attachments']['sequences']), 'release_animations': len(payload['attachments']['animations']),
                           'effect_types': len(effects['types']), 'effect_sequences': len(effects['sequences']),
                           'attached_sequences': len(payload['attached_effects']['sequences']),
                           'fighter0_projectile_frames': len(fighter0_projectiles['animations']),
                           'fighter2_projectile_frames': len(fighter2_projectiles['animations']),
                           'fighter1_projectile_frames': len(fighter1_projectiles['animations']),
                           'fighter4_projectile_frames': len(fighter4_projectiles['animations']),
                           'fighter5_projectile_frames': len(fighter5_projectiles['frames']),
                           'fighter7_children_frames': len(fighter7_children['frames']),
                           'fighter4_curve_words': len(fighter4_projectiles['curve']['values']),
                           'alternate_movements': len(alternate_movements),
                           'arena_resources': len(arena_resources),
                           'arena_linear_rows': len(arena_linear['linear_rows']),
                           'arena_linear_words': sum(len(row) for row in arena_linear['linear_rows'].values()),
                           'arena4_rows': len(arena_four['arena4_rows']),
                           'arena4_words': sum(len(row) for row in arena_four['arena4_rows'].values()),
                           'arena4_palette_words': sum(len(row) for row in arena_four['arena4_colors']),
                           'fighter0_release_sequences': len(fighter0_projectiles['sequences'])},
                'bytes': len(raw), 'gzip_bytes': len(packed), 'sha256': sha(raw), 'gzip_sha256': sha(packed)}
    (ROOT / data / 'chu-world-rules-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps({key: value for key, value in manifest.items() if key != 'sources'}, indent=2))


if __name__ == '__main__':
    main()
