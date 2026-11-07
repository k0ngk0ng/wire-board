"""Prepare source-bound Japanese stages 6/7 dispatch and private REA inputs.

Failure inventory before reconstruction: US addresses mistaken for Japanese;
state tables ending at shared routines miscounted; first-stage properties or
supertile requests reused for mechanical tile animations; hidden global writes
lost; queue rejection advances damage early; multislot ownership drifts.
"""
import hashlib,json,zipfile
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-enemies-machines'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'

def main():
    before=hashlib.sha256(ROM.read_bytes()).hexdigest()
    with zipfile.ZipFile(ROM)as z:raw=z.read('Contra (J).nes')
    assert hashlib.sha256(raw).hexdigest()==SHA
    prg=raw[16:131088];OUT.mkdir(parents=True,exist_ok=True)
    image=prg[7:0x4000]+prg[0x1c000:]
    assert prg[7:11].hex()=='a9809d98'
    target=OUT/'machines-window.bin';target.write_bytes(image)
    word=lambda at:int.from_bytes(prg[at:at+2],'little')
    stages=[];seeds={}
    for level,kinds in [(5,5),(6,9)]:
        root=word(0x1e5c9+level*2);entries=[]
        for kind in range(16,16+kinds):
            table=word(root+0x10000+(kind-16)*2);offset=table-0x8000
            first=word(offset+2);count=(first-(table+2))//2
            assert 1<=count<=16 and (first-table-2)%2==0
            addresses=[word(offset+state*2)for state in range(1,count+1)]
            assert all(0x8007<=a<=0xffff for a in addresses)
            entries.append({'kind':kind,'table_minus_two':hex(table),'count':count,'routines':[hex(a)for a in addresses]})
            for state,a in enumerate(addresses,1):seeds.setdefault(a,f'level{level+1}_type{kind:02x}_state{state}')
        stages.append({'level':level,'table':hex(root),'entries':entries})
    seed=OUT/'dispatch-seeds.tsv';seed.write_text(''.join(f'{hex(a)}\tfunction\t{name}\n'for a,name in seeds.items()))
    spec={'target':str(target),'entry':'0x8007','seeds':str(seed),'output':str(OUT/'dispatch-rea'),
          'addresses':[hex(a)for a in seeds],'file_sha256':hashlib.sha256(image).hexdigest(),
          'tmp':str(ROOT/'dist/contra-enemies-machines-tmp'),'cache':str(OUT/'swift-cache')}
    (OUT/'dispatch-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    manifest={'rom_sha256':SHA,'archive_sha256':before,'window':{'cpu_start':'0x8007','cpu_end':'0xffff','banks':'0/1 at8000-BFFF and14/15 atC000-FFFF','sha256':spec['file_sha256']},'stages':stages,
              'limitations':['Table extents inferred from first routine boundary and then cross-checked through original dispatch.', 'This inventory is not full gameplay verification.']}
    assert hashlib.sha256(ROM.read_bytes()).hexdigest()==before
    (OUT/'dispatch.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps({'stages':2,'types':14,'entries':len(seeds)}))

if __name__=='__main__':main()
