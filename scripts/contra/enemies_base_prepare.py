"""Identify Japanese base-stage enemy tables and private REA windows.

Failure inventory: US address/RAM reuse, shared-function pointers mistaken for
table ends, stage-one properties reused in bases, missing room progression
globals, and bank-switched data interpreted under the wrong loaded window.
"""
from pathlib import Path
import hashlib,json,zipfile
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-base'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
NAMES=['boss-eye','roller','grenade','wall-turret','wall-core','indoor-soldier','jumping-soldier','grenade-launcher','four-soldiers','soldier-generator','roller-generator','eye-projectile','boss-gemini','spinning-bubbles','blue-soldier','red-soldier','red-blue-generator']

def main():
    before=hashlib.sha256(ROM.read_bytes()).hexdigest()
    with zipfile.ZipFile(ROM)as z:rom=z.read('Contra (J).nes')
    digest=hashlib.sha256(rom).hexdigest();assert digest=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    p=rom[16:131088];word=lambda a:int.from_bytes(p[a:a+2],'little')
    assert word(0x1e5cb)==word(0x1e5cf)==0xe5ff
    assert word(0x1edf5)==word(0x1edf9)==0xee11
    BASE.mkdir(parents=True,exist_ok=True)
    data=p[7:0x4000]+p[0x1c000:];target=BASE/'base-enemies.bin';target.write_bytes(data)
    tables=[];seeds={}
    for i,name in enumerate(NAMES):
        table=word(0x1e5ff+i*2);candidates=[word(table-0x8000+j*2)for j in range(1,17)]
        first=min(a for a in candidates if table+2<=a<0xc000);count=(first-table-2)//2
        assert 1<=count<=16 and (first-table-2)%2==0
        routines=candidates[:count]
        for phase,address in enumerate(routines,1):
            assert 0x8007<=address<=0xffff
            seeds.setdefault(address,f'base_{i+16:02x}_{phase:02x}')
        prop=p[0x1ee11+(i+16)*4:0x1ee15+(i+16)*4]
        tables.append({'kind':i+16,'name':name,'table_minus_two':hex(table),'routines':[hex(a)for a in routines],'properties':list(prop),'property_rom_offset':16+0x1ee11+(i+16)*4})
    # Room loader is in bank4/5; it needs its own window, never enemy bank0.
    spawn=p[0xaa11:0xc000]+p[0x1c000:];spawnpath=BASE/'base-room-spawn.bin';spawnpath.write_bytes(spawn)
    seeds.update({0xe668:'base_clear_room_or_boss',0xe5ba:'load_level_enemy_table',0xedad:'initialize_enemy'})
    seed=BASE/'base-seeds.tsv';seed.write_text(''.join(f'{hex(a)}\tfunction\t{name.replace("-","_")}\n'for a,name in seeds.items()))
    # Keep the first request focused on room/core/generator plus Boss entries.
    selected={16,19,20,25,26,28,32}
    addresses=list(dict.fromkeys([a for t in tables if t['kind']in selected for a in t['routines']]+['0xe668','0xedad']))
    spec={'target':str(target),'entry':'0x8007','seeds':str(seed),'output':str(BASE/'core-rea'),'tmp':str(ROOT/'dist/contra-enemies-base-tmp'),'cache':str(BASE/'swift-cache'),'addresses':addresses,'file_sha256':hashlib.sha256(data).hexdigest()}
    (BASE/'core-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    spawnseed=BASE/'spawn-seeds.tsv';spawnseed.write_text('0xaa11\tfunction\tload_base_room\n0xedad\tfunction\tinitialize_enemy\n')
    (BASE/'spawn-spec.json').write_text(json.dumps(dict(spec,target=str(spawnpath),entry='0xaa11',seeds=str(spawnseed),output=str(BASE/'spawn-rea'),addresses=['0xaa11','0xedad'],file_sha256=hashlib.sha256(spawn).hexdigest()),indent=2)+'\n')
    extra=[a for t in tables if t['kind']in{17,18,21,22,23,24,30,31}for a in t['routines']]
    extra+=['0xeb70','0xeb0a','0xeb1b','0xe949','0xea11','0xea78','0xea95','0xec5b','0x91d5','0x971e','0xa367','0xe6bf','0x8e86','0x9f3a']
    plans={
      'extra':list(dict.fromkeys(extra)),
      'helper':['0xea98','0xea23','0xea7e','0xeb6e','0xe6d5','0xe6db','0x97ef'],
      'soldier-helper':['0x969b','0x96c5','0x9325','0x979f','0x975e','0x971b','0x951a','0x9680','0xecb4','0x95a1','0x95ab','0xeccc','0x82d4','0xe879','0xe882','0xeaa8'],
      'final-helper':['0xa1e0','0xa1f6','0xeaa6'],
      'common':['0xef2d','0xef3a','0xef6d','0xefae','0xefd3','0xefeb','0xeff0','0xf016','0xf017'],
      'pickup':['0x8100','0xed03','0xeccc'],
    }
    for name,addresses in plans.items():
        ownseed=seed
        if name=='common':
            ownseed=BASE/'common-seeds.tsv';ownseed.write_text(seed.read_text()+''.join(a+'\tfunction\tbase_common_'+a[2:]+'\n'for a in addresses))
        if name=='pickup':
            ownseed=BASE/'pickup-seeds.tsv';ownseed.write_text(seed.read_text()+'0x8100\tfunction\tbase_pickup_rest\n0xed03\tfunction\tbase_pickup_velocity\n')
        (BASE/(name+'-spec.json')).write_text(json.dumps(dict(spec,seeds=str(ownseed),output=str(BASE/(name+'-rea')),addresses=addresses),indent=2)+'\n')
    rooms={}
    for level,count in [(1,6),(3,9)]:
        table=word(0xaa75+level*2);rows=[]
        for screen in range(count):
            start=word(table+screen*2);at=start+1;items=[]
            while p[at]!=255:
                pos,kind,attr=p[at:at+3];items.append({'position':pos,'type_flags':kind,'kind':kind&63,'attributes':attr});at+=3;assert at-start<160
            rows.append({'screen':screen,'cpu':hex(start),'rom_offset':start+16,'core_count':p[start],'items':items,'bytes':list(p[start:at+1])})
        rooms[str(level)]={'table':hex(table),'entries':rows}
    result={'rom_sha256':digest,'archive_sha256':before,'mapping':'bank0/1 at8000 plus fixed14/15 atC000; source table identities matched on Japanese bytes','tables':tables,'rooms':rooms,'shared_dependencies':[8,10],
            'notes':['Table lengths end at first local code target; backward targets are shared routines, not table lengths.','Runtime/RAM semantics still require original execution checks.']}
    assert hashlib.sha256(ROM.read_bytes()).hexdigest()==before
    (ROOT/'docs/research/nes-contra/evidence/enemies-base-inventory.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'types':len(tables),'core_functions':len(spec['addresses']),'room_counts':{k:len(v['entries'])for k,v in rooms.items()}}))

if __name__=='__main__':main()
