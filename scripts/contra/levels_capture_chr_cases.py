"""Original CHR function boundary probes with explicit, one-time input writes.

Cases precede any implementation of the CHR state machine: transition clearing,
countdown gates, Base Boss/Waterfall disable, snow pace carry, lair pace wrap,
screen-12 secondary table, and secondary animation after alternate graphics.
"""
import gzip,json,os,shutil,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-levels/chr-cases'))
from capture_reference import BASE,run
from levels_capture_chr import HOOK

CASES=[
 ('transition',1,{0x6a:1,0x5c:17,0x71:2}),
 ('countdown',1,{0x6a:0,0x5c:1}),
 ('jungle-delay',1,{0x6a:0,0x5c:0,0x34:7,0x71:0}),
 ('base-boss',2,{0x6a:0,0x40:128,0x5c:0}),
 ('waterfall-alternate',3,{0x6a:0,0x48:255,0x5c:0}),
 ('snow-pace-carry',5,{0x6a:0,0x34:3,0x5d:255,0x5c:0}),
 ('lair-pace',8,{0x6a:0,0x5d:1,0x5c:0}),
 ('lair-pace-wrap',8,{0x6a:0,0x5d:255,0x64:11,0x5c:0}),
 ('lair-screen12-wait',8,{0x6a:0,0x5d:0,0x64:12,0x1a:1,0x5c:0}),
 ('lair-screen12-row8',8,{0x6a:0,0x5d:0,0x64:12,0x1a:8,0x5c:0,0x71:3}),
 ('lair-screen12-alternate',8,{0x6a:0,0x5d:0,0x64:12,0x1a:8,0x5c:0,0x71:3,0x48:255}),
 ('lair-screen13-wrap',8,{0x6a:0,0x5d:0,0x64:13,0x5c:0}),
 ('energy-no-animation',6,{0x6a:0,0x5c:0}),
 ('hangar-no-animation',7,{0x6a:0,0x5c:0}),
]


def main():
 label=sys.argv[1]if len(sys.argv)>1 else 'v1';(BASE/'states').mkdir(parents=True,exist_ok=True);observations=[]
 for name,stage,changes in CASES:
  state=f'stage-{stage}-v2';src=ROOT/'.local/contra-campaign/states'/(state+'.fcs');dst=BASE/'states'/(state+'.fcs')
  if dst.exists():assert src.read_bytes()==dst.read_bytes()
  else:shutil.copyfile(src,dst)
  writes='\n'.join(f'memory.writebyte({address},{value})'for address,value in changes.items())
  hook='local controlled_done=false\n'+HOOK.replace('-- CONTROLLED_INPUT',f'if not controlled_done then\n{writes}\ncontrolled_done=true\nend')
  spec={'name':f'chr-case-{name}-{label}','load_state':state,'frames':2,'captures':[],'segments':[],'hooks_lua':hook}
  p=BASE/(spec['name']+'.json');p.write_text(json.dumps(spec,indent=2)+'\n');out=run(p)
  calls=[json.loads(s)for s in (out/'chr-animation.jsonl').read_text().splitlines()];assert len(calls)==2
  observations.append({'name':name,'stage':stage,'changes':[{'address':a,'value':v}for a,v in changes.items()],
                       'source':json.loads((out/'source.json').read_text()),'calls':calls})
 target=BASE/f'chr-cases-{label}.json.gz'
 target.write_bytes(gzip.compress(json.dumps({'scope':'Controlled function inputs at C83B; no natural campaign claim.',
                                             'observations':observations},separators=(',',':')).encode(),mtime=0))
 print(json.dumps({'cases':len(observations),'calls':sum(len(o['calls'])for o in observations),'file':str(target)}))


if __name__=='__main__':main()
