"""Naturally break the first M box, pick it up, then hold B to fire.

The initial state was reached by normal title/menu input. This recording uses
controller input only; its hooks never write game memory.
"""
import os
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-weapons/pickup'))
from capture_reference import BASE,run
from weapons_capture import COMMON
import argparse,gzip,hashlib,json,shutil

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--name',required=True);parser.add_argument('--output',type=Path,default=ROOT/'docs/research/nes-contra/evidence');args=parser.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    source=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if not target.exists():shutil.copyfile(source,target)
    assert target.read_bytes()==source.read_bytes()
    segments=[]
    for actor in [1,2]:
        segments += [dict(start=1,end=500,player=actor,keys=['right']),dict(start=220,end=270,player=actor,keys=['down'])]
        segments += [dict(start=n,end=n,player=actor,keys=['A']) for n in [15,85,155,470,540,610]]
        segments += [dict(start=n,end=n,player=actor,keys=['B']) for n in range(1,351,4)]
        segments += [dict(start=351,end=700,player=actor,keys=['B'])]
    spec={'name':args.name,'load_state':'coop-play','frames':700,'segments':segments,'captures':[220,275,310,340,380,420,500,600,700],'hooks_lua':COMMON.replace('CASES','local cases=nil')}
    path=BASE/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    ram=(out/'ram.bin').read_bytes()
    pickups=[{'frame':frame+1,'actor':actor,'weapon':ram[frame*2048+0xaa+actor]} for frame in range(700) for actor in [0,1] if ram[frame*2048+0xaa+actor]&15==1]
    assert pickups and pickups[0]=={'frame':292,'actor':0,'weapon':1}
    rows=[json.loads(line) for line in (out/'weapons.jsonl').read_text().splitlines()]
    result={'scope':'machine','interventions':None,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'pickup':{'first':pickups[0],'framesWithMachinegun':len(pickups),'ram_sha256':hashlib.sha256(ram).hexdigest()},'fire':[r for r in rows if r['kind']=='fire'],'steps':[r for r in rows if r['kind']=='step']}
    args.output=args.output.resolve();assert args.output.is_relative_to(ROOT);args.output.mkdir(parents=True,exist_ok=True)
    fixture=args.output/'weapons-machine.json.gz';temporary=fixture.with_suffix('.tmp');temporary.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0));temporary.replace(fixture)
    print(json.dumps({'fire':len(result['fire']),'steps':len(result['steps']),'pickup':result['pickup'],'sha256':hashlib.sha256(fixture.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
