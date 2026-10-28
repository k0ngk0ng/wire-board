"""Prepare a source-bound Japanese sprite bank slice for private REA analysis.

Failures considered: wrong ROM, incorrect PRG bank mapping, importing data as an
entry, table address drift, out-of-bank pointers, and writes outside workspace.
"""
from pathlib import Path
import hashlib
import json
import zipfile

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / '.local/contra-sprites'
ROM_SOURCE = Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
ROM_SHA = '6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def main():
    before = hashlib.sha256(ROM_SOURCE.read_bytes()).hexdigest()
    with zipfile.ZipFile(ROM_SOURCE) as z:
        rom = z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest() == ROM_SHA
    prg = rom[16:16+131072]
    # The original NMI calls $919F after mapping bank 4/5. Its backward branch
    # reaches the verified instruction entry $9167, so the slice starts there.
    assert prg[0x919f:0x91a6].hex() == 'adeb074a90c2a9'
    assert prg[0x9167:0x916b].hex() == 'a9048510'
    blob = prg[0x9167:0xc000]
    OUT.mkdir(parents=True, exist_ok=True)
    target = OUT/'sprites-9167.bin'
    target.write_bytes(blob)
    seeds = {'0x9167':'draw_sprites_linear','0x919f':'draw_sprites',
             '0x9284':'compose_meta_sprite','0x936e':'draw_hud',
             '0xa4f8':'update_player_sprite_and_attributes','0xa4fb':'update_player_attributes',
             '0xa551':'choose_player_sprite','0xa581':'choose_large_sequence',
             '0xa583':'choose_walking_sprite','0xa5ec':'choose_death_sprite',
             '0xa625':'choose_falling_sprite','0xa629':'apply_facing',
             '0xa638':'choose_jumping_sprite','0xa677':'choose_water_sprite',
             '0xa7a1':'choose_indoor_sprite','0xa7fc':'choose_indoor_boss_sprite'}
    seed_file = OUT/'sprite-seeds.tsv'
    seed_file.write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in seeds.items()))
    spec = {'target':str(target),'entry':'0x9167','seeds':str(seed_file),
            'output':str(OUT/'sprite-rea'),'addresses':list(seeds),
            'file_sha256':hashlib.sha256(blob).hexdigest()}
    (OUT/'sprite-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    pointers = []
    for code in range(1,256):
        offset = 0x93d8+code*2 if code<128 else 0x94d8+(code&127)*2
        pointer = int.from_bytes(prg[offset:offset+2], 'little')
        assert 0x95d8 <= pointer < 0xc000, (code,pointer)
        pointers.append({'code':code,'cpu_address':pointer,'rom_offset':16+pointer,
                         'header':prg[pointer]})
    manifest = {'rom_sha256':ROM_SHA,'zip_sha256':before,'prg_banks':[4,5],
                'slice':{'cpu_start':0x9167,'cpu_end':0xbfff,'rom_offset':16+0x9167,
                         'bytes':len(blob),'sha256':spec['file_sha256']},
                'sprite_pointer_tables':[0x93da,0x94d8],'sprites':pointers,
                'source_unchanged':before==hashlib.sha256(ROM_SOURCE.read_bytes()).hexdigest()}
    assert manifest['source_unchanged']
    (OUT/'source-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps({'spec':str(OUT/'sprite-spec.json'),'sprites':len(pointers)}))


if __name__ == '__main__':
    main()
