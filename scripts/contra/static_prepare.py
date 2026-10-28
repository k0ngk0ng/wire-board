#!/usr/bin/env python3
"""Prepare identified Japanese PRG banks for bounded static REA analysis.

Failure boundaries: wrong ROM/version, changed input, trainer/trailing bytes,
confused 8/16 KiB banks, bogus load origin, or loss of source-offset identity.
Only research copies are written; no executable rules are generated here.
"""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / '.local/contra-static'
ROM = Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA = '6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def main():
    before = sha(ROM.read_bytes())
    with zipfile.ZipFile(ROM) as archive:
        assert archive.testzip() is None
        raw = archive.read('Contra (J).nes')
    assert sha(raw) == SHA
    assert raw[:16].hex() == '4e45531a081070100000000000000000'
    assert len(raw) == 16 + 128*1024 + 128*1024
    prg = raw[16:16+128*1024]
    OUT.mkdir(parents=True, exist_ok=True)
    banks = []
    for index in range(16):
        data = prg[index*8192:(index+1)*8192]
        name = f'bank-{index:02}.bin'
        (OUT/name).write_bytes(data)
        banks.append({'bank_8k': index, 'prg_offset': index*8192,
                      'rom_offset': 16+index*8192, 'bytes': len(data),
                      'sha256': sha(data), 'local_file': name})
    # NMI at $F889 contains JSR $C001. Excluding byte $C000 ensures that REA's
    # required raw load/entry address is a demonstrated instruction boundary.
    fixed = prg[14*8192+1:]
    (OUT/'fixed-c001.bin').write_bytes(fixed)
    seeds = {
        '0xc001': 'frame_dispatch', '0xc1bd': 'load_controllers',
        '0xc1f9': 'read_controller_ports', '0xf80d': 'reset',
        '0xf889': 'nmi', '0xf8e1': 'irq', '0xf989': 'set_prg_pair',
        '0xf98f': 'restore_prg_pair',
    }
    (OUT/'fixed-seeds.tsv').write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in seeds.items()))
    spec = {'target': str(OUT/'fixed-c001.bin'), 'entry': '0xc001',
            'seeds': str(OUT/'fixed-seeds.tsv'), 'output': str(OUT/'fixed-rea'),
            'addresses': list(seeds), 'file_sha256': sha(fixed)}
    (OUT/'fixed-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    physics = {
        '0xcc42': 'level_dispatch', '0xd2d9': 'clear_frame_scroll',
        '0xd3ca': 'player_state_dispatch', '0xd3ed': 'player_spawn',
        '0xd45e': 'player_normal', '0xd495': 'player_normal_logic',
        '0xd4bd': 'player_death_animation',
        '0xd518': 'update_player_state_and_x', '0xd525': 'apply_player_x',
        '0xd595': 'handle_player_state', '0xd5fd': 'jump_if_not_airborne',
        '0xd601': 'set_jump_velocity', '0xd626': 'handle_dpad',
        '0xd6ad': 'handle_edge_fall', '0xd6dd': 'handle_jump',
        '0xd848': 'set_player_aim', '0xd8b0': 'check_ledge',
        '0xd8fe': 'gravity_and_y_position', '0xd901': 'apply_player_y_position',
        '0xd922': 'apply_gravity', '0xd930': 'death_reduce_lives',
        '0xd941': 'check_frame_scroll', '0xd9c7': 'check_drop_down',
        '0xda5e': 'check_solid_bg_collision',
    }
    (OUT/'physics-seeds.tsv').write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in physics.items()))
    physics_spec = dict(spec, seeds=str(OUT/'physics-seeds.tsv'),
                        output=str(OUT/'physics-rea'), addresses=list(physics))
    (OUT/'physics-spec.json').write_text(json.dumps(physics_spec,indent=2)+'\n')
    terrain = {
        '0xdd4b': 'load_tile_column', '0xdd97': 'expand_supertiles_column',
        '0xdf30': 'classify_and_write_tile_collision',
        '0xdfb8': 'collision_with_floor_solid_check',
        '0xdfe0': 'read_collision_with_lower_floor',
        '0xdfe6': 'read_collision_unchecked', '0xdfec': 'query_bg_collision',
        '0xe067': 'boss_auto_scroll', '0xe09c': 'decode_screen_supertiles',
    }
    (OUT/'terrain-seeds.tsv').write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in terrain.items()))
    terrain_spec = dict(spec, seeds=str(OUT/'terrain-seeds.tsv'),
                        output=str(OUT/'terrain-rea'), addresses=list(terrain))
    (OUT/'terrain-spec.json').write_text(json.dumps(terrain_spec,indent=2)+'\n')
    header_slice = prg[5*8192+0x816:6*8192]
    (OUT/'header-a816.bin').write_bytes(header_slice)
    (OUT/'header-seeds.tsv').write_text('0xa816\tfunction\tload_level_header\n')
    header_spec = {'target': str(OUT/'header-a816.bin'), 'entry':'0xa816',
                   'seeds':str(OUT/'header-seeds.tsv'), 'output':str(OUT/'header-rea'),
                   'addresses':['0xa816'], 'file_sha256':sha(header_slice)}
    (OUT/'header-spec.json').write_text(json.dumps(header_spec,indent=2)+'\n')
    report = {'rom_sha256': SHA, 'zip_sha256': before, 'banks': banks,
              'fixed_slice': {'prg_offset': 14*8192+1, 'rom_offset':16+14*8192+1,
                              'cpu_start':'0xc001','cpu_end':'0xffff','bytes':len(fixed),'sha256':sha(fixed)},
              'mapping_limit': 'C000/E000 final PRG windows are a static mapping assumption pending execution evidence.',
              'source_unchanged': sha(ROM.read_bytes()) == before}
    assert report['source_unchanged']
    (OUT/'bank-manifest.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'passed': True, 'banks': len(banks), 'spec': str(OUT/'fixed-spec.json')}))


if __name__ == '__main__':
    main()
