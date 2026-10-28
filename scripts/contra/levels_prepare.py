"""Prepare bounded, source-verified REA slices for Japanese level loading."""
from pathlib import Path
import hashlib
import json
import zipfile

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-levels'
ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def main():
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()==ROM_SHA
    prg=rom[16:16+131072];OUT.mkdir(parents=True,exist_ok=True)
    specs=[('fixed',0xc001,prg[0x1c001:],{
        '0xc001':'original_frame_entry','0xc76e':'select_alternate_chr',
        '0xc778':'select_chr_preset','0xcec6':'load_alternate_palette_and_collision',
        '0xcee3':'queue_palette_colors','0xdcc5':'swap_base_boss_map_pointers',
        '0xdd16':'enter_base_boss_scene','0xe09c':'decode_level_map',
        '0xc3d7':'request_supertile_update','0xface':'apply_chr_mapping',
    }),('header',0xa816,prg[0xa816:0xc000],{'0xa816':'load_stage_header'}),
    ('animation',0xc001,prg[0x1c001:],{
        '0xc001':'original_frame_entry','0xc858':'chr_transition_blank',
        '0xc867':'chr_animation_return','0xc868':'chr_jungle_delay',
        '0xc871':'chr_base_delay','0xc879':'chr_waterfall_delay',
        '0xc886':'chr_snow_delay','0xc892':'chr_lair_delay',
        '0xc8ab':'advance_chr_animation',
    })]
    for name,entry,blob,seeds in specs:
        target=OUT/f'{name}.bin';target.write_bytes(blob)
        seed=OUT/f'{name}-seeds.tsv';seed.write_text(''.join(f'{a}\tfunction\t{n}\n'for a,n in seeds.items()))
        spec={'target':str(target),'entry':hex(entry),'seeds':str(seed),'output':str(OUT/f'{name}-rea'),
              'addresses':list(seeds),'file_sha256':hashlib.sha256(blob).hexdigest()}
        (OUT/f'{name}-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    print(json.dumps({'rom_sha256':ROM_SHA,'specs':[s[0]for s in specs]}))


if __name__=='__main__':main()
