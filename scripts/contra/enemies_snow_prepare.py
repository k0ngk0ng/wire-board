"""Source-bound Japanese snow-field enemy dispatch for private REA analysis.

Failure inventory: using US state addresses, wrong banks/property tables,
mistaking background tanks for sprite motion, delayed shared scroll/palette
effects, multi-slot spawn order, queue refusal and completion shortcuts.
"""
from pathlib import Path
import hashlib,json,zipfile

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-enemies-snow'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'

def main():
    before=hashlib.sha256(ROM.read_bytes()).hexdigest()
    with zipfile.ZipFile(ROM)as z:rom=z.read('Contra (J).nes')
    assert hashlib.sha256(rom).hexdigest()==SHA
    p=rom[16:131088];table=int.from_bytes(p[0x1e5d1:0x1e5d3],'little');assert table==0xe62d
    names=['ice_generator','ice_grenade','tank','ice_joint','carrier','mini_ufo','dropped_bomb']
    seeds={};types=[]
    for i,name in enumerate(names):
        at=0x10000+table+i*2;pointer=int.from_bytes(p[at:at+2],'little');offset=pointer-0x8000
        first=int.from_bytes(p[offset+2:offset+4],'little');count=(first-(pointer+2))//2
        assert 1<=count<=16 and (first-(pointer+2))%2==0
        routines=[]
        for state in range(1,count+1):
            address=int.from_bytes(p[offset+state*2:offset+state*2+2],'little');assert 0x8007<=address<=0xffff
            routines.append(address);seeds.setdefault(address,f'snow_{name}_{state}')
        types.append({'kind':16+i,'name':name,'table':pointer,'states':routines})
    OUT.mkdir(parents=True,exist_ok=True)
    blob=p[7:0x4000]+p[0x1c000:];target=OUT/'enemy-window.bin';target.write_bytes(blob)
    seed=OUT/'seeds.tsv';seed.write_text(''.join(f'{hex(a)}\tfunction\t{name}\n'for a,name in seeds.items()))
    spec={'target':str(target),'entry':'0x8007','seeds':str(seed),'output':str(OUT/'rea'),
          'addresses':[hex(a)for a in seeds],'file_sha256':hashlib.sha256(blob).hexdigest(),
          'tmp':str(ROOT/'dist/contra-enemies-snow-tmp'),'cache':str(OUT/'swift-cache')}
    (OUT/'spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    manifest={'rom_sha256':SHA,'source_archive_sha256':before,'level':4,'dispatch_table':hex(table),
              'mapping':'bank0/1 at 8000-BFFF, fixed14/15 at C000-FFFF; leading data excluded at 8007',
              'types':types,'window_sha256':spec['file_sha256']}
    (OUT/'dispatch.json').write_text(json.dumps(manifest,indent=2)+'\n')
    assert hashlib.sha256(ROM.read_bytes()).hexdigest()==before
    print(json.dumps({'types':len(types),'entries':len(seeds),'spec':str(OUT/'spec.json')}))

if __name__=='__main__':main()
