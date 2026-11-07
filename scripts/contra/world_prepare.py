"""Prepare source-bound frame orchestration for a private REA session.

Failure boundaries: moving a frame/timer phase before its original call,
confusing display-buffer generation with simulation, or reusing US addresses.
This prepares research slices and seeds; it emits no game implementation.
"""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-world'


def main():
    source=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
    before=hashlib.sha256(source.read_bytes()).hexdigest()
    with zipfile.ZipFile(source) as archive:rom=archive.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    target=rom[16+14*8192+1:16+131072]
    BASE.mkdir(parents=True,exist_ok=True)
    (BASE/'fixed-c001.bin').write_bytes(target)
    entries={'0xcccc':'normal_level_frame','0xd296':'players_projectiles_frame',
             '0xd37e':'select_camera_follower','0xcc5f':'initialize_level'}
    (BASE/'frame-seeds.tsv').write_text(''.join(f'{pc}\tfunction\t{name}\n' for pc,name in entries.items()))
    spec={'target':str(BASE/'fixed-c001.bin'),'entry':'0xc001','seeds':str(BASE/'frame-seeds.tsv'),
          'file_sha256':hashlib.sha256(target).hexdigest(),'addresses':list(entries),'output':str(BASE/'frame-rea'),
          'tmp':str(ROOT/'dist/cw-tmp'),'cache':str(BASE/'swift-cache')}
    (BASE/'frame-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    display={'0xd2b0':'alternate_projectile_sprites','0xfa0a':'update_projectile_bank','0xfa3a':'update_terrain_scroll_bank'}
    (BASE/'display-seeds.tsv').write_text(''.join(f'{pc}\tfunction\t{name}\n' for pc,name in (entries|display).items()))
    display_spec=dict(spec,seeds=str(BASE/'display-seeds.tsv'),addresses=list(display),output=str(BASE/'display-rea'))
    (BASE/'display-spec.json').write_text(json.dumps(display_spec,indent=2)+'\n')
    (BASE/'scroll-seeds.tsv').write_text('0xdb81\tfunction\thandle_terrain_scroll\n')
    scroll_spec=dict(spec,seeds=str(BASE/'scroll-seeds.tsv'),addresses=['0xdb81'],output=str(BASE/'scroll-rea'))
    (BASE/'scroll-spec.json').write_text(json.dumps(scroll_spec,indent=2)+'\n')
    stream={'0xce59':'pre_player_frame','0xce94':'pause_control','0xc83b':'background_chr_animation',
            '0xcf6d':'palette_cycle','0xe08c':'next_horizontal_screen','0xdd4b':'write_background_column',
            '0xdd97':'expand_supertile_column','0xde79':'write_background_attributes'}
    (BASE/'stream-seeds.tsv').write_text(''.join(f'{pc}\tfunction\t{name}\n' for pc,name in stream.items()))
    stream_spec=dict(spec,seeds=str(BASE/'stream-seeds.tsv'),addresses=list(stream),output=str(BASE/'stream-rea'))
    (BASE/'stream-spec.json').write_text(json.dumps(stream_spec,indent=2)+'\n')
    assert hashlib.sha256(source.read_bytes()).hexdigest()==before
    (BASE/'source.json').write_text(json.dumps({'romSHA256':hashlib.sha256(rom).hexdigest(),'sourceUnchanged':True,
        'slice':{'cpuStart':'0xc001','cpuEnd':'0xffff','romOffset':16+14*8192+1,'sha256':spec['file_sha256']}},indent=2)+'\n')
    print(BASE/'frame-spec.json')


if __name__=='__main__':main()
