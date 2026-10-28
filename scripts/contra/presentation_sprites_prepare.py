"""Prepare original whole-scene sprite composition evidence and descriptors."""
from pathlib import Path
import gzip,hashlib,json,zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-presentation-sprites'
ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def main():
 with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
 assert hashlib.sha256(rom).hexdigest()==ROM_SHA
 prg=rom[16:16+131072];BASE.mkdir(parents=True,exist_ok=True)
 raw=prg[0x9167:0xc000];target=BASE/'composition.bin';target.write_bytes(raw)
 seeds={'0x9167':'compose_linear','0x919f':'compose_scene','0x91f8':'compose_snow',
        '0x926a':'advance_snow_oam','0x9284':'compose_meta_sprite','0x936e':'compose_lives_hud'}
 seed=BASE/'seeds.tsv';seed.write_text(''.join(f'{a}\tfunction\t{n}\n'for a,n in seeds.items()))
 spec={'target':str(target),'entry':'0x9167','seeds':str(seed),'output':str(BASE/'rea'),
       'addresses':list(seeds),'file_sha256':hashlib.sha256(raw).hexdigest()}
 (BASE/'spec.json').write_text(json.dumps(spec,indent=2)+'\n')
 print(json.dumps({'spec':str(BASE/'spec.json'),'romSHA256':ROM_SHA}))


if __name__=='__main__':main()
