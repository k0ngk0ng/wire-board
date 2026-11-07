"""Prepare exact Japanese shooting/aim functions for isolated REA sessions.

Failures considered before implementation: reference-region address drift,
unverified angle/speed tables, lost carry/byte behavior, wrong 8 KiB mappings,
and accepting inferred behavior without an original execution oracle.
"""
from pathlib import Path
import hashlib,json,zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-shooters'

def main():
    path=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
    before=hashlib.sha256(path.read_bytes()).hexdigest()
    with zipfile.ZipFile(path)as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    prg=rom[16:131088];BASE.mkdir(parents=True,exist_ok=True)
    # A verified first enemy instruction is $8007. Remaining code windows map
    # original PRG banks0/1 at $8000 and fixed14/15 at $C000.
    data=prg[7:16384]+prg[0x1c000:]
    target=BASE/'enemy-window.bin';target.write_bytes(data)
    addresses=[0xf225,0xf29a,0xf30b,0xf324,0xf329,0xf330,0xf339,0xf34a,0xf34b,0xf356,0xf365,0xf37b,
               0xf3b3,0xf3b7,0xf492,0xf4c4,0x8b11]
    seed=BASE/'aim-seeds.tsv';seed.write_text(''.join(f'{hex(a)}\tfunction\tshooting_{a:04x}\n'for a in addresses))
    spec={'target':str(target),'entry':'0x8007','seeds':str(seed),'output':str(BASE/'aim-rea'),
          'tmp':str(ROOT/'dist/contra-enemies-shooters-tmp'),'cache':str(BASE/'swift-cache'),
          'addresses':[hex(a)for a in addresses],'file_sha256':hashlib.sha256(data).hexdigest()}
    (BASE/'aim-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    assert hashlib.sha256(path.read_bytes()).hexdigest()==before
    print(json.dumps({'spec':str(BASE/'aim-spec.json')}))

if __name__=='__main__':main()
