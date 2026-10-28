"""Original indoor pickup trajectory and controlled scroll/flight boundaries."""
from pathlib import Path
import os,gzip,json,hashlib,shutil,sys
from enemies_base_capture import FIELDS,WORLD,HOOK as NATURAL
from enemies_base_boss_capture import HOOK as CONTROLLED,lua
import capture_reference
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-base/pickup-oracle'
capture_reference.BASE=BASE;os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
WF=WORLD+[('screenScroll',0x65)]

def main():
 label=sys.argv[1]if len(sys.argv)>1 else 'v1';mode=sys.argv[2]if len(sys.argv)>2 else 'controlled';cases=[]
 (BASE/'states').mkdir(parents=True,exist_ok=True);source=ROOT/'.local/contra-campaign/states/stage-2-v2.fcs';target=BASE/'states/base.fcs'
 if not target.exists():shutil.copyfile(source,target)
 assert source.read_bytes()==target.read_bytes()
 if mode=='controlled':
  for scene in [1,128]:
   for phase in [1,2,3]:
    for x in [7,8,95,107,108,115,116,123,124,131,132,139,140,147,148,255]:
     for scroll in [0,1,3]:
      for kind in [0,6]:
       world=dict(frame=8,weaponStrength=0,level=1,gameLoop=0,random=19,boss=0,sceneType=scene,vertical=0,screen=0,roomCleared=0,frameScroll=0,indoorScroll=0,cores=1,plates=0,rounds=0,redCreated=0,grenadeLauncher=0,attackEnabled=1,readOffset=0,delayLow=0,delayHigh=0,scrollY=224,scrollX=0,control=176,screenScroll=scroll)
       cases.append(dict(kind=0,routine=phase,full=0,values=[1,109,x,0,phase,250,250,1,0,0,128,0,0,0,0xfd,0,0xf0,0x22,0x80,kind,109,250,255,128],world=[world[k]for k,_ in WF],players=[[1,64,176,0],[1,192,176,0]]))
  hook=CONTROLLED.replace('restore={},audio={}','restore={},audio={},requests={}')
  hook=hook.replace("local function snap()","local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end\nlocal function snap()")
  hook=hook.replace("..table.concat(ps,',')..']}'end","..table.concat(ps,',')..'],\"queueUsed\":'..read(0x21)..',\"queue\":\"'..hex(0x700,128)..'\",\"supertiles\":\"'..hex(0x600,128)..'\",\"collision\":\"'..hex(0x680,128)..'\"}'end")
  hook=hook.replace('CASES',lua(cases));frames=len(cases)
 else:hook=NATURAL.replace('SETUP','');frames=1600
 spec=dict(name=f'base-pickup-{mode}-{label}',load_state='base',frames=frames,segments=[],captures=[],hooks_lua=hook.replace('FIELDS',lua(FIELDS)).replace('WORLD',lua([a for _,a in WF])),timeout_seconds=180)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=capture_reference.run(path)
 rows=[json.loads(x)for x in(out/('boss.jsonl'if mode=='controlled'else'base-enemies.jsonl')).read_text().splitlines()]
 if mode=='controlled':
  assert len(rows)==len(cases)
  for r in rows:r.update(id=r['case'],type=0,slot=15,name='step',requests=[])
 else:rows=[r for r in rows if r['name']=='step'and r['type']==0]
 data=dict(source_is_original_execution=True,source=json.loads((out/'source.json').read_text()),scenario=spec,cases=cases,records=rows,world_fields=[dict(name=k,address=hex(a))for k,a in WF])
 dest=ROOT/f'docs/research/nes-contra/evidence/enemies-base-pickup-{mode}.json.gz';packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
 if dest.exists():
  old=json.loads(gzip.decompress(dest.read_bytes()));assert old['records']==rows,'repeat differs'
 else:dest.write_bytes(packed)
 print(json.dumps(dict(mode=mode,records=len(rows),sha256=hashlib.sha256(dest.read_bytes()).hexdigest())))
if __name__=='__main__':main()
