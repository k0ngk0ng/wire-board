"""Prepare the source-bound fixed-bank graphics routines for private REA.

Only verified instruction entries are seeded. Mapper/low-bank data are not
emulated; level data identities and actual call behavior are verified separately.
"""
import hashlib,json,zipfile
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-enemies-terrain'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')

def main():
    before=hashlib.sha256(ROM.read_bytes()).hexdigest()
    with zipfile.ZipFile(ROM)as z:r=z.read('Contra (J).nes')
    assert hashlib.sha256(r).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    prg=r[16:131088]
    assert prg[0x1c3d1:0x1c3d9].hex()=='8511a521c940b0f7'
    assert prg[0x1c656:0x1c65c].hex()=='a5102980850f'
    OUT.mkdir(parents=True,exist_ok=True)
    blob=prg[0x1c001:];target=OUT/'fixed-c001.bin';target.write_bytes(blob)
    entries={a:f'graphics_{a:04x}'for a in (0xc3d1,0xc656,0xc6d7)}
    seeds=OUT/'seeds.tsv';seeds.write_text(''.join(f'{hex(a)}\tfunction\t{label}\n'for a,label in entries.items()))
    spec={'target':str(target),'entry':'0xc001','seeds':str(seeds),'output':str(OUT/'rea'),
          'addresses':[hex(a)for a in entries],'file_sha256':hashlib.sha256(blob).hexdigest(),
          'tmp':str(ROOT/'dist/contra-enemies-terrain-tmp'),'cache':str(OUT/'swift-cache')}
    (OUT/'spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    assert hashlib.sha256(ROM.read_bytes()).hexdigest()==before
    print(json.dumps({'spec':str(OUT/'spec.json'),'source_unchanged':True}))

if __name__=='__main__':main()
