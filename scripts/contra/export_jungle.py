"""Export only decoded stage-one map definitions and bounded compressed screens.

Failures checked before game data export: wrong ROM identity, source range/hash
drift, alias confusion, bad supertile IDs and accidentally exporting executable
cartridge banks. Original gameplay execution remains in the oracle, not here.
"""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parents[2]
ROM = Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')


def main():
    source_path = ROOT/'docs/research/nes-contra/evidence/static-level1-terrain.json'
    source = json.loads(source_path.read_text())
    before = hashlib.sha256(ROM.read_bytes()).hexdigest()
    with zipfile.ZipFile(ROM) as archive:
        raw = archive.read('Contra (J).nes')
    assert hashlib.sha256(raw).hexdigest() == source['rom_sha256'] == '6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    screens = []
    for screen in source['screens']:
        location = screen['source']
        start, length = location['rom_offset'], location['bytes']
        encoded = raw[start:start+length]
        assert len(encoded) == length and hashlib.sha256(encoded).hexdigest() == location['sha256']
        assert all(0 <= n < len(source['supertiles']) for n in screen['supertiles'])
        screens.append({'encoded': encoded.hex(), 'alias': screen['alias_of']})
    assert len(screens) == 14 and screens[13]['alias'] == 0
    assert screens[13]['encoded'] == screens[0]['encoded']
    data = {'schema': 1, 'romSHA256': source['rom_sha256'], 'screenRows': 7,
            'distinctScreens': source['distinct_screens'], 'lookaheadAlias': 13,
            'automaticScrollScreen': source['header_stop_scroll_screen'],
            'alternateGraphicsScreen': source['header_alternate_graphics_screen'],
            'collisionLimits': source['collision_limits'], 'screens': screens,
            'supertiles': [{'tiles': s['tiles'], 'attribute': s['attribute']} for s in source['supertiles']]}
    out = ROOT/'games/contra/engine/leveldata'
    out.mkdir(parents=True, exist_ok=True)
    packed = (json.dumps(data, separators=(',', ':'))+'\n').encode()
    (out/'jungle.json').write_bytes(packed)
    manifest = {'source': str(source_path.relative_to(ROOT)), 'sourceSHA256': hashlib.sha256(source_path.read_bytes()).hexdigest(),
                'romSHA256': source['rom_sha256'], 'outputSHA256': hashlib.sha256(packed).hexdigest(),
                'scope': 'Stage-one terrain definitions only; no CPU opcodes, original player runtime, or other stages.',
                'screenByteRanges': [s['source'] for s in source['screens']]}
    (out/'jungle-source.json').write_text(json.dumps(manifest, indent=2)+'\n')
    assert hashlib.sha256(ROM.read_bytes()).hexdigest() == before
    print(json.dumps({'output': str((out/'jungle.json').relative_to(ROOT)), 'bytes': len(packed), 'sha256': manifest['outputSHA256']}))


if __name__ == '__main__':
    main()
