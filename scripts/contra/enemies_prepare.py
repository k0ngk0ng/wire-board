"""Prepare Japanese enemy analysis windows; reference labels never choose bytes.

Failure cases before analysis: wrong region, ambiguous byte signatures, wrong
bank windows, table bytes mistaken for code, unfixed source identity, or edits
outside the workspace. Native execution fixtures precede rules implementation.
"""
from pathlib import Path
import hashlib
import json
import zipfile

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-enemies'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'

def sha(b):return hashlib.sha256(b).hexdigest()

def main():
    source=sha(ROM.read_bytes())
    with zipfile.ZipFile(ROM) as z:r=z.read('Contra (J).nes')
    assert sha(r)==SHA
    p=r[16:131088];OUT.mkdir(parents=True,exist_ok=True)
    assert p[0x1e54f:0x1e557].hex()=='a90085b685b7a20f'
    assert p[0xa97b:0xa97f].hex()=='a5300aa8'
    assert p[0xaa85:0xaa8a].hex()=='a564cd9501'
    common={i:int.from_bytes(p[0x1e5d9+i*2:0x1e5db+i*2],'little') for i in range(16)}
    special=[int.from_bytes(p[0x1e5f9+i*2:0x1e5fb+i*2],'little') for i in range(3)]
    base={0xe54f:'dispatch_all_enemies',0xe58a:'dispatch_enemy',0xe5ba:'load_level_enemy_dispatch',
          0xe10a:'enemy_player_collision',0xe276:'player_bullet_enemy_collision',
          0xed34:'find_free_enemy_slot',0xed3f:'find_enemy_bullet_slot',
          0xed62:'clear_enemy',0xed70:'clear_enemy_kinematics',0xedad:'initialize_enemy',
          0xfa1a:'enemy_update_wrapper',0xfa22:'screen_spawn_wrapper',0xfa32:'soldier_spawn_wrapper'}
    enemy=dict(base)
    # Each positive state indexes a two-byte entry after table-minus-two.
    tables={**common,**{0x10+i:t for i,t in enumerate(special)}}
    routines={}
    for kind in [0,1,2,3,4,5,6,7,0x10,0x11,0x12]:
        table=tables[kind];offset=table-0x8000
        first=int.from_bytes(p[offset+2:offset+4],'little')
        count=(first-(table+2))//2
        assert 1<=count<=16 and (first-(table+2))%2==0
        targets=[]
        for state in range(1,count+1):
            address=int.from_bytes(p[offset+state*2:offset+state*2+2],'little')
            if 0x8007<=address<0xc000 or 0xc000<=address<=0xffff:
                targets.append(address);enemy.setdefault(address,f'enemy_{kind:02x}_state_{state:02x}')
        routines[str(kind)]={'table':hex(table),'candidate_states':[hex(a) for a in targets],
                             'limitation':'State table ends at its first routine entry in these Japanese bytes; actual dispatch coverage comes from original execution.'}
    windows=[('enemy',0x8007,p[7:0x4000]+p[0x1c000:],enemy),
             ('spawn',0xa97b,p[0xa97b:0xc000]+p[0x1c000:],{**base,0xa97b:'spawn_screen_enemies',0xaa11:'spawn_indoor_screen',0xaa85:'spawn_soldiers',0xaa9f:'soldier_generation_init',0xaae3:'soldier_generation_test',0xabb9:'soldier_generation_create'})]
    specs=[]
    for name,entry,data,seeds in windows:
        target=OUT/f'{name}.bin';target.write_bytes(data)
        seed_path=OUT/f'{name}-seeds.tsv';seed_path.write_text(''.join(f'{hex(a)}\tfunction\t{label}\n' for a,label in seeds.items()))
        spec={'target':str(target),'entry':hex(entry),'seeds':str(seed_path),'output':str(OUT/f'{name}-rea'),
              'addresses':[hex(a) for a in seeds],'file_sha256':sha(data)}
        path=OUT/f'{name}-spec.json';path.write_text(json.dumps(spec,indent=2)+'\n');specs.append(str(path))
    # These are verified instruction entries or branch/call targets, not guessed
    # table locations. Keep the smaller semantic fragments independently replayable.
    groups={
      'helper':('enemy',[0xe6bc,0xe6bf,0xe6c8,0xe73a,0xe74b,0xe768,0xe847,0xe879,0xe882,0xea6d,0xea7c,0xea85,0xea8d,0xea98,0xeaa6,0xeaa8,0xeaee,0xeb1b,0xeb8d,0xeb99,0xeb9b,0xebb1,0xebd5,0xec5b,0xf224,0xf225,0xf3b7,0x82d4,0x871c,0x87a3,0x87d4,0x87da,0x8929,0x8b11]),
      'runner-valid-checked':('enemy',[0x8647,0x864d,0x866b,0x87e8,0xe86d,0xe879,0xe882,0xea7c,0xeaa6,0xeaee,0xeb8d,0xeb99,0xeb9b,0x8719,0xe67c,0xe6c8]),
      'small':('enemy',[0xebf9,0xf9e4,0xe6e1,0xe737]),
      'terrain-known':('enemy',[0xc3d1,0xf997,0xeb1b,0x82a4,0x84ab,0xea19,0xea95,0xed57]),
      'pickup-checked':('enemy',[0x811c,0xe7d2,0x80c9,0x80d0,0x80e3,0xeaf0]),
      'bridge':('enemy',[0xe989,0x8c77,0x8cff,0xea1d]),
      'soldier':('spawn',[0xaaaa,0xaab1,0xab68,0xab8a,0xac43,0xac92,0xac97,0xacea,0xed30]),
    }
    for name,(window,addresses) in groups.items():
        spec=json.loads((OUT/f'{window}-spec.json').read_text())
        seed=OUT/f'{name}-seeds.tsv';seed.write_text(''.join(f'{hex(a)}\tfunction\tcode_{a:04x}\n'for a in addresses))
        spec.update(seeds=str(seed),output=str(OUT/f'{name}-rea'),addresses=[hex(a)for a in addresses])
        path=OUT/f'{name}-spec.json';path.write_text(json.dumps(spec,indent=2)+'\n');specs.append(str(path))
    report={'rom_sha256':SHA,'archive_sha256':source,'windows':{'enemy':'bank0/1 at $8000-$BFFF plus fixed14/15 at $C000-$FFFF; leading table bytes excluded',
             'spawn':'bank4/5 at $8000-$BFFF plus fixed14/15; slice begins at verified $A97B'},
            'common_tables':{str(k):hex(v) for k,v in common.items()},'stage_one_tables':[hex(v) for v in special],'routine_candidates':routines}
    assert sha(ROM.read_bytes())==source
    (OUT/'identity.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'specs':specs}))

if __name__=='__main__':main()
