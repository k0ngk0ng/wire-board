"""Capture JP Base bosses from explicitly staged original dispatch calls.
Failure inventory: plate threshold and stopped delay; byte carry; damage layers;
full child pool; mirrored blink; delayed collision; RNG write; dead-player aim;
projectile visibility/collision threshold; curved homing tie and wrap; defeat
reset/audio ordering; final delay before campaign transition.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-base/boss-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_base_capture import FIELDS,WORLD
# importing base capture changes its environment only; restore private root.
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
HOOK=r'''
local f=assert(io.open(out..'/boss.jsonl','w'));local fields=FIELDS;local wf=WORLD;local cases=CASES;local index=0;local pending=nil;local returns={}
local function read(a)return memory.readbyte(a)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1 end
local function arr(aa,offset)local t={};for _,a in ipairs(aa)do t[#t+1]=tostring(read(a+offset))end;return '['..table.concat(t,',')..']'end
local function snap()local p={};for i=0,15 do p[#p+1]=arr(fields,i)end;local ps={};for i=0,1 do ps[#ps+1]=string.format('{"state":%d,"x":%d,"y":%d,"gameOver":%d}',read(0x90+i),read(0x334+i),read(0x31a+i),read(0x38+i))end;return '{"generationBias":'..read(6)..',"pool":['..table.concat(p,',')..'],"world":'..arr(wf,0)..',"players":['..table.concat(ps,',')..']}'end
local function arm(pc)if returns[pc]then return end;returns[pc]=true;memory.registerexec(pc,1,function()if not pending or pending.pc~=pc then return end;local p=pending;pending=nil;f:write(string.format('{"case":%d,"before":%s,"after":%s,"audio":[%s],"biasWrites":[%s]}\n',p.index,p.before,snap(),table.concat(p.audio,','),table.concat(p.biasWrites,',')));for a,v in pairs(p.restore)do memory.writebyte(a,v)end end)end
memory.registerwrite(6,1,function(a,n,v)if pending then pending.biasWrites[#pending.biasWrites+1]=string.format('{"pc":%d,"value":%d}',memory.getregister('pc'),v)end end)
memory.registerexec(0xf9d4,1,function()if pending then pending.audio[#pending.audio+1]='{"kind":"reset"}'end end)
memory.registerexec(0xf9bc,1,function()if pending then pending.audio[#pending.audio+1]=string.format('{"kind":"play","code":%d}',memory.getregister('a'))end end)
memory.registerexec(0xe54f,1,function()index=index+1;assert(cases[index],'unexpected frame');for i=0,15 do memory.writebyte(0x4b8+i,0)end;memory.writebyte(0x528+15,cases[index].kind);memory.writebyte(0x4b8+15,1)end)
memory.registerexec(0xe58a,1,function()
 if memory.getregister('x')~=15 then return end;assert(not pending)
 local c=cases[index];local p={index=index,pc=ret(),restore={},audio={},biasWrites={}}
 local function set(a,v)if p.restore[a]==nil then p.restore[a]=read(a)end;memory.writebyte(a,v)end
 for slot=0,15 do for _,a in ipairs(fields)do set(a+slot,0)end end
 if c.full~=0 then for i=0,14 do set(0x4b8+i,2);set(0x528+i,1);set(0x30a+i,1);set(0x578+i,2)end end
 for i,a in ipairs(fields)do set(a+15,c.values[i])end
 for i,a in ipairs(wf)do set(a,c.world[i])end
 for i=0,1 do local pp=c.players[i+1];set(0x90+i,pp[1]);set(0x334+i,pp[2]);set(0x31a+i,pp[3]);set(0x38+i,pp[4])end
 BIAS_SETUP
 set(4,c.routine*2);p.before=snap();pending=p;arm(p.pc)
end)
function finish_probe()assert(not pending);f:close()end
'''
def lua(v):
 if isinstance(v,dict):return '{'+','.join(k+'='+lua(x)for k,x in v.items())+'}'
 if isinstance(v,list):return '{'+','.join(lua(x)for x in v)+'}'
 return str(v)
def main():
 label=sys.argv[1]if len(sys.argv)>1 else 'v1';bias='--bias'in sys.argv[2:];cases=[]
 def add(kind,routine,**kw):
  c=dict(kind=kind,routine=routine,delay=1,attack=1,frame=0,x=128,y=48,vx=320,vy=0,fracX=0,fracY=0,attr=0,width=15,hp=1,data=[16,0,0,10],tick=0,enabled=1,plates=4,cores=2,random=255,strength=0,full=0,players=[[1,64,176,0],[1,192,176,0]])
  c.update(kw);cases.append(c)
 for kind,last in [(16,7),(27,5),(28,7),(29,5)]:
  for routine in range(1,last+1):
   for delay in [0,1,2,64]:add(kind,routine,delay=delay)
 for kind in [16,28]:
  for plates in range(5):
   for delay in [0,1,2]:add(kind,2,plates=plates,delay=delay)
  for n in [0,1,2,6,7,10,16]:
   for delay in [0,1,2]:add(kind,4,data=[n,0,0,n],delay=delay)
  for cores in [0,1,2]:
   for full in [0,1]:add(kind,5,cores=cores,full=full);add(kind,7,cores=cores,full=full)
 for tick in [0,1,4,7,8,15,16,255]:
  for delay in [0,1,2]:
   for full in [0,1]:
    for kind in [16,28]:add(kind,3,tick=tick,delay=delay,full=full,strength=tick%4,random=tick,data=[128,2,1,10])
 for x in [0,8,79,80,175,176,255]:
  for vx in [-320,320]:add(16,3,x=x,vx=vx,fracX=250)
 for offset in [0,1,47,48,49,255]:
  for vx in [-128,128]:
   for tick in [0,1]:add(28,3,vy=offset*256+250,vx=vx,tick=tick,delay=0,attack=2,data=[128,0,0,10])
 for kind in [27,29]:
  for tick in range(4):
   for p1 in [[1,64,176,0],[1,128,16,0],[3,0,0,1]]:
    for p2 in [[1,192,176,0],[1,128,240,0],[3,0,0,1]]:add(kind,1,tick=tick,players=[p1,p2])
 for y in [0,47,48,71,72,73,231,232,255]:
  for tick in range(16):add(27,2,y=y,tick=tick,vy=256,attr=165,width=143)
 for attr in range(4):
  for delay in [0,1,2,3,4,7,8,255]:
   for angle in [0,1,6,11,12,13,18,23]:add(29,2,attr=attr,delay=delay,data=[angle,0,0,0],frame=5)
 for count in [19,20,21,255]:
  for attack in [0,1,2]:add(29,2,data=[23,1,count,0],attack=attack)
 for c in cases:
  vx=c['vx']&65535;vy=c['vy']&65535
  c['values']=[1,c['y'],c['x'],2,c['routine'],c['fracY'],c['fracX'],vy>>8,vy&255,vx>>8,vx&255,c['kind'],c['delay'],0,c['attack'],c['frame'],c['hp'],2,c['width'],c['attr']]+c['data']
  w=dict(frame=c['tick'],weaponStrength=c['strength'],level=3 if c['kind']in[28,29]else 1,gameLoop=0,random=c['random'],boss=0,sceneType=128,vertical=0,screen=5,roomCleared=0,frameScroll=0,indoorScroll=0,cores=c['cores'],plates=c['plates'],rounds=0,redCreated=0,grenadeLauncher=0,attackEnabled=c['enabled'],readOffset=0,delayLow=17,delayHigh=3,scrollY=224,scrollX=0,control=176)
  c['world']=[w[k]for k,_ in WORLD]
 (BASE/'states').mkdir(parents=True,exist_ok=True);source=ROOT/'.local/contra-campaign/states/stage-2-v2.fcs';target=BASE/'states/base.fcs'
 if not target.exists():shutil.copyfile(source,target)
 assert source.read_bytes()==target.read_bytes()
 spec=dict(name='base-boss-'+label,load_state='base',frames=len(cases),segments=[],captures=[],hooks_lua=HOOK.replace('FIELDS',lua(FIELDS)).replace('WORLD',lua([a for _,a in WORLD])).replace('CASES',lua(cases)).replace('BIAS_SETUP','set(6,83)'if bias else''),timeout_seconds=180)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 rows=[json.loads(x)for x in(out/'boss.jsonl').read_text().splitlines()];assert len(rows)==len(cases),(len(rows),len(cases))
 if not bias:
  for r in rows:r.pop('biasWrites');r['before'].pop('generationBias');r['after'].pop('generationBias')
 data=dict(source_is_original_execution=True,scope='controlled dispatch; declared inputs restored after each original call; not natural boss traversal',source=json.loads((out/'source.json').read_text()),scenario=spec,world_fields=[dict(name=k,address=hex(a))for k,a in WORLD],cases=cases,records=rows)
 if bias:data['generation_bias_setup']=83
 dest=ROOT/('docs/research/nes-contra/evidence/enemies-base-boss-bias.json.gz'if bias else'docs/research/nes-contra/evidence/enemies-base-boss-controlled.json.gz');packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
 if dest.exists():
  old=json.loads(gzip.decompress(dest.read_bytes()));assert old['cases']==cases and old['records']==rows,'repeat differs'
 else:dest.write_bytes(packed)
 print(json.dumps(dict(cases=len(rows),sha256=hashlib.sha256(dest.read_bytes()).hexdigest())))
if __name__=='__main__':main()
