#!/usr/bin/env python3
"""Prepare source-bound fixed-bank and animation-bank scene analysis."""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-static'


def main():
    spec=json.loads((BASE/'fixed-spec.json').read_text())
    entries={'0xd63c':'base_up','0xd651':'base_partner_gate','0xd665':'base_advance_both',
             '0xd762':'electrocution','0xd771':'base_transition_motion','0xd78a':'base_transition_end',
             '0xd7d1':'base_transition_velocity','0xd803':'speed_to_fixed','0xd8d9':'scene_floor',
             '0xd825':'scene_landing','0xd941':'scene_camera','0xd3ed':'scene_spawn','0xd4bd':'scene_death'}
    (BASE/'modes-seeds.tsv').write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in entries.items()))
    spec.update(seeds=str(BASE/'modes-seeds.tsv'),output=str(BASE/'modes-rea'),addresses=list(entries))
    (BASE/'modes-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    source=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
    source_digest=hashlib.sha256(source.read_bytes()).hexdigest()
    with zipfile.ZipFile(source) as archive:raw=archive.read('Contra (J).nes')
    assert hashlib.sha256(raw).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    data=raw[16+0xa4f8:16+0xc000];target=BASE/'base-sprite-a4f8.bin';target.write_bytes(data)
    entries={'0xa7a1':'base_animation_dispatch','0xa7b6':'base_walking','0xa7bf':'base_sequence_seven',
             '0xa7c3':'base_standing','0xa7c7':'base_shock_pose','0xa7cb':'base_crouched','0xa7d0':'base_depth_walk',
             '0xa7e9':'base_death_pose','0xa7fc':'base_boss_animation_dispatch','0xa80b':'base_boss_standing'}
    (BASE/'mode-sprites-seeds.tsv').write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in entries.items()))
    spec={'target':str(target),'entry':'0xa4f8','file_sha256':hashlib.sha256(data).hexdigest(),
          'seeds':str(BASE/'mode-sprites-seeds.tsv'),'output':str(BASE/'mode-sprites-rea'),'addresses':list(entries)}
    (BASE/'mode-sprites-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    assert hashlib.sha256(source.read_bytes()).hexdigest()==source_digest
    print(json.dumps({'fixed_functions':13,'sprite_functions':10,'source_unchanged':True}))


if __name__=='__main__':main()
