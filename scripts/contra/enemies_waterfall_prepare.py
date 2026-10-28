"""Bind Japanese waterfall enemy tables/functions to an explicit REA window.

Failures fixed before reconstruction: US addresses or shared-kind assumptions;
wrong vertical scroll sign; floating support mistaken for damage; rock spawn
reuse; Dragon link/rotation/HP state lost; early Boss completion; and reporting
controlled stage entry as a natural campaign clear.
"""
from pathlib import Path
import hashlib,json,zipfile

ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-waterfall'
def sha(b):return hashlib.sha256(b).hexdigest()

def main():
    src=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip');before=sha(src.read_bytes())
    with zipfile.ZipFile(src)as z:r=z.read('Contra (J).nes')
    assert sha(r)=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    p=r[16:131088];BASE.mkdir(parents=True,exist_ok=True)
    tables=[];seeds={0x8007:'shared_first_entry',0xe54f:'enemy_phase',0xe58a:'enemy_dispatch',0xe671:'shared_boss_defeated',0xea50:'boss_defeated_globals',0xe67c:'explosion_start',0xe6e1:'explosion_update',0xe745:'clear_sprite_advance',0xea19:'create_two_explosions'}
    for index,count in enumerate([2,2,3,6,9,8]):
        table=int.from_bytes(p[0x1e621+2*index:0x1e623+2*index],'little')
        states=[int.from_bytes(p[table-0x8000+2*state:table-0x8000+2*state+2],'little')for state in range(1,count+1)]
        assert all(0x8007<=a<0xc000 or 0xc001<=a<=0xffff for a in states)
        for state,address in enumerate(states,1):seeds.setdefault(address,f'waterfall_{0x10+index:02x}_state_{state}')
        tables.append({'kind':0x10+index,'table':hex(table),'tableROMOffset':16+table-0x8000,'states':[hex(a)for a in states]})
    blob=p[7:0x4000]+p[0x1c000:];target=BASE/'enemy-window.bin';target.write_bytes(blob)
    seedpath=BASE/'seeds.tsv';seedpath.write_text(''.join(f'{hex(a)}\tfunction\t{name}\n'for a,name in seeds.items()))
    spec={'target':str(target),'entry':'0x8007','seeds':str(seedpath),'output':str(BASE/'rea'),'addresses':[hex(a)for a in seeds],
          'file_sha256':sha(blob),'tmp':str(ROOT/'dist/contra-enemies-waterfall-tmp'),'cache':str(BASE/'swift-cache')}
    (BASE/'spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    groups={
      'details':[0xeabd,0x98d1,0x98e3,0xef34,0xea6d,0xea7c,0x9956,0xea5c,0xe882,0xeaa6,0xeb99,0x9a4e,0x9991,0x9ae0,0x9b7e,0x9c1e,0x9ef8],
      'arm':[0x9b3f,0x9bda,0x9c11,0x9c39,0x9da5,0x9e69,0xeb34,0xea19,0xea8d,0xea98,0xe8d8,0xf433],
      'arm-detail':[0x9d03,0x9d23,0x9dc7,0xf492,0xea69,0xea78,0xeb1d],
      'scuba':[0xf0ad,0xf0b2,0xf0e9,0xf204,0xf4c4],
    }
    for name,addresses in groups.items():
        seed=BASE/(name+'-seeds.tsv');seed.write_text(''.join(f'{hex(a)}\tfunction\twaterfall_{name.replace("-","_")}_{a:04x}\n'for a in addresses))
        child=dict(spec,seeds=str(seed),output=str(BASE/(name+'-rea')),addresses=[hex(a)for a in addresses])
        (BASE/(name+'-spec.json')).write_text(json.dumps(child,indent=2)+'\n')
    assert sha(src.read_bytes())==before
    (BASE/'identity.json').write_text(json.dumps({'romSHA256':sha(r),'archiveSHA256':before,'window':'bank0/1 at 8000-BFFF and fixed14/15 at C000-FFFF; leading 7 table bytes excluded','targetSHA256':sha(blob),'dispatchTable':'fixed:E621','types':tables},indent=2)+'\n')
    print(json.dumps({'spec':str(BASE/'spec.json'),'functions':len(seeds),'types':tables}))

if __name__=='__main__':main()
