"""Original weapon trajectories with an explicitly staged equipment choice.

Only current-weapon bytes AA/AB are set once after a source-bound initial state.
No positions, ammunition slots, timers or later frames are patched. This covers
weapon rules under original execution, not natural pickup or campaign clearing.
"""
from weapons_capture import ROOT,BASE,COMMON
from capture_reference import run
import gzip,hashlib,json,shutil,argparse

def main():
    parser=argparse.ArgumentParser();parser.add_argument('label',nargs='?',default='equipment-v1');parser.add_argument('--output',type=type(ROOT),default=ROOT/'docs/research/nes-contra/evidence');args=parser.parse_args();label=args.label
    fire=[];steps=[];sources=[]
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    for stage in [1,2,3]:
        origin=ROOT/'.local/contra-campaign/states'/f'stage-{stage}-v2.fcs'
        target=BASE/'states'/f'stage-{stage}.fcs'
        if not target.exists():shutil.copyfile(origin,target)
        assert origin.read_bytes()==target.read_bytes()
        for weapon in [0,1,2,3,4,16,17,18,19,20]:
            segments=[dict(start=1,end=180,player=i,keys=['B']) for i in [1,2]]
            segments += [dict(start=n,end=n+1,player=i,keys=['A']) for i in [1,2] for n in [10,70,130]]
            # Releasing B creates new edges for semi-automatic weapons.
            if weapon&15 not in [1,4]:
                segments=[s for s in segments if s['keys']!=['B']]+[dict(start=n,end=n,player=i,keys=['B']) for i in [1,2] for n in range(1,181,4)]
            segments += [dict(start=1,end=30,player=1,keys=['right']),dict(start=31,end=60,player=2,keys=['left']),dict(start=61,end=90,player=1,keys=['up']),dict(start=91,end=120,player=2,keys=['up','left']),dict(start=121,end=150,player=1,keys=['down'])]
            name=f'{label}-s{stage}-w{weapon}'
            hooks=COMMON.replace('CASES',f'local cases=nil;memory.writebyte(0xaa,{weapon});memory.writebyte(0xab,{weapon})')
            spec={'name':name,'load_state':f'stage-{stage}','frames':180,'segments':segments,'captures':[],'hooks_lua':hooks}
            path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
            rows=[json.loads(x) for x in (out/'weapons.jsonl').read_text().splitlines()]
            fire.extend(dict(r,scenario=name) for r in rows if r['kind']=='fire');steps.extend(dict(r,scenario=name) for r in rows if r['kind']=='step')
            sources.append({'scenario':name,'stage':stage,'weapon':weapon,'source':json.loads((out/'source.json').read_text()),'timeline':segments,'trace_sha256':hashlib.sha256((out/'weapons.jsonl').read_bytes()).hexdigest()})
    data={'scope':'equipment','interventions':'Once after load: AA/AB set to selected weapon. Stage entries are controlled first-initialization captures, not a naturally cleared campaign. No further patches.','fire':fire,'steps':steps,'sources':sources}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
    args.output=args.output.resolve()
    assert args.output.is_relative_to(ROOT)
    args.output.mkdir(parents=True,exist_ok=True)
    target=args.output/'weapons-equipment.json.gz';temporary=target.with_suffix('.tmp');temporary.write_bytes(packed);temporary.replace(target)
    print(json.dumps({'fire':len(fire),'steps':len(steps),'sha256':hashlib.sha256(packed).hexdigest()}))

if __name__=='__main__':main()
