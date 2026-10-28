"""Controlled Japanese base rules, with actual graphics queue admission.

Failure inventory: queue refusal/retry; terminal room timing; full allocation;
odd/even generator phase; exhausted loops; damage layers; redraw idempotence;
byte-zero timers; small/large throw animation; soldier death/pickup conversion.
"""
from pathlib import Path
import os,gzip,hashlib,json,shutil,sys
from enemies_base_capture import FIELDS,WORLD
from enemies_base_boss_capture import HOOK,lua
from capture_reference import run
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-base/cases'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
import capture_reference
capture_reference.BASE=BASE

def main():
 label=sys.argv[1]if len(sys.argv)>1 else 'v1';mode=sys.argv[2]if len(sys.argv)>2 else 'general';cases=[]
 def add(kind,routine,**kw):
  c=dict(kind=kind,routine=routine,level=3 if kind in[30,31,32]else 1,delay=1,attack=1,frame=0,x=128,y=104,vx=-192,vy=0,fracX=0,fracY=0,attr=0,width=19,hp=8,data=[0,0,0,0],tick=1,enabled=1,plates=0,cores=2,rounds=0,red=0,launcher=0,random=255,strength=0,full=0,used=0,screen=0,players=[[1,64,176,0],[1,192,176,0]])
  c.update(kw);cases.append(c)
 for kind,last in [(8,8),(10,7),(17,5),(18,6),(19,8),(20,10),(21,7),(22,8),(23,7),(24,8),(25,3),(26,3),(30,7),(31,6),(32,3)]:
  for routine in range(1,last+1):
   for delay in [0,1,2,16]:
    add(kind,routine,delay=delay,used=0)
 for kind in [19,20]:
  for routine in range(1,6):
   for used in [0,47,48,79,80,126]:
    for attr in ([0,1,2,3]if kind==19 else [0,1,4,8,12]):add(kind,routine,used=used,attr=attr)
 for routine in [8,9,10]:
  for count in [1,2]:
   for full in [0,1]:
    for used in [0,48,80]:add(20,routine,cores=count,full=full,used=used,data=[0,0,3,0])
 for kind in [25,26,32]:
  for tick in [0,1,2,3]:
   for full in [0,1]:
    for enabled in [0,1]:
     for rounds in [0,6,7]:add(kind,2,tick=tick,full=full,enabled=enabled,rounds=rounds)
 for kind in [21,22,23,24]:
  for attr in range(8):
   for x in [79,80,95,96,103,104,127,151,152,159,160,175,176]:add(kind,2,attr=attr,x=x,delay=8,attack=1)
 for attr in range(8):
  for rounds in [0,1]:
   for red in [0,1]:add(22,1,attr=attr,rounds=rounds,red=red)
 for kind in [8,10]:
  for routine in range(1,6):
   for attr in [0,1,2,3]:
    for used in [0,48,80]:add(kind,routine,attr=attr,used=used)
 if mode=='waves':
  # Every reachable room/program cursor, including the second roller program.
  # Inputs come from the pinned Japanese data, never from generated Go output.
  import zipfile
  with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
  assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
  prg=rom[16:131088];get=lambda a,n:prg[a-0x8000:a-0x8000+n];word=lambda a:int.from_bytes(get(a,2),'little');cases=[]
  for group,rooms in [(0,5),(1,8)]:
   table=word(0x8dde+group*2)
   for screen in range(rooms):
    start=word(table+screen*2);cursor=0
    while True:
     for rounds in [0,6]:
      for full in [0,1]:add(25,2,level=1+group*2,attr=group,screen=screen,rounds=rounds,full=full,data=[cursor,0,0,0])
     if get(start+cursor+1,1)[0]&128:break
     cursor+=2;assert cursor<100
  for group in range(2):
   start=word(0x9634+group*2);cursor=0
   while True:
    for full in [0,1]:
     for enabled in [0,1]:add(26,2,level=1+group*2,attr=group,screen=3 if group==0 else 5,full=full,enabled=enabled,data=[cursor,0,0,0])
    if get(start+cursor,1)[0]==255:break
    cursor+=2;assert cursor<100
  for cursor in range(28):
   for full in [0,1]:add(32,2,tick=0,full=full,data=[cursor,0,0,0])
 for c in cases:
  vx=c['vx']&65535;vy=c['vy']&65535
  c['values']=[1,c['y'],c['x'],2,c['routine'],c['fracY'],c['fracX'],vy>>8,vy&255,vx>>8,vx&255,c['kind'],c['delay'],0,c['attack'],c['frame'],c['hp'],2,c['width'],c['attr']]+c['data']
  w=dict(frame=c['tick'],weaponStrength=c['strength'],level=c['level'],gameLoop=0,random=c['random'],boss=0,sceneType=128 if c['kind']in[8,10,30,31,32]else 1,vertical=0,screen=c['screen'],roomCleared=0,frameScroll=0,indoorScroll=0,cores=c['cores'],plates=c['plates'],rounds=c['rounds'],redCreated=c['red'],grenadeLauncher=c['launcher'],attackEnabled=c['enabled'],readOffset=0,delayLow=17,delayHigh=3,scrollY=224,scrollX=0,control=176)
  c['world']=[w[k]for k,_ in WORLD]
 # Extend the already verified controlled dispatcher, keeping full raw queue
 # bytes and restoring them at the original return boundary.
 hook=HOOK.replace("local function snap()","local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end\nlocal function snap()")
 hook=hook.replace("..table.concat(ps,',')..']}'end","..table.concat(ps,',')..'],\"queueUsed\":'..read(0x21)..',\"queue\":\"'..hex(0x700,128)..'\",\"supertiles\":\"'..hex(0x600,128)..'\",\"collision\":\"'..hex(0x680,128)..'\",\"scratch0A\":'..read(0x0a)..'}'end")
 hook=hook.replace('restore={},audio={}','restore={},audio={},requests={}')
 hook=hook.replace('"audio":[%s]}','"audio":[%s],"requests":[%s]}').replace("table.concat(p.audio,',')))","table.concat(p.audio,','),table.concat(p.requests,',')))")
 hook=hook.replace('set(4,c.routine*2);',"set(0x21,c.used);set(0x0a,19);for a=0x600,0x67f do set(a,0)end;for a=0x680,0x6ff do set(a,0)end;for a=0x700,0x77f do set(a,0)end;set(4,c.routine*2);")
 hook+="\nmemory.registerexec(0xc3d1,1,function()if pending then pending.requests[#pending.requests+1]=string.format('{\"kind\":\"supertile\",\"x\":%d,\"y\":%d,\"code\":%d,\"used\":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end end)\nmemory.registerexec(0xc593,1,function()if pending then pending.requests[#pending.requests+1]=string.format('{\"kind\":\"tiles\",\"x\":%d,\"y\":%d,\"code\":%d,\"used\":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end end)\n"
 (BASE/'states').mkdir(parents=True,exist_ok=True);source=ROOT/'.local/contra-campaign/states/stage-2-v2.fcs';target=BASE/'states/base.fcs'
 if not target.exists():shutil.copyfile(source,target)
 assert source.read_bytes()==target.read_bytes()
 spec=dict(name='base-'+('waves'if mode=='waves'else'cases')+'-'+label,load_state='base',frames=len(cases),segments=[],captures=[],hooks_lua=hook.replace('FIELDS',lua(FIELDS)).replace('WORLD',lua([a for _,a in WORLD])).replace('CASES',lua(cases)),timeout_seconds=180)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 rows=[json.loads(x)for x in(out/'boss.jsonl').read_text().splitlines()];assert len(rows)==len(cases),(len(rows),len(cases))
 for row in rows:row.update(id=row['case'],slot=15,type=cases[row['case']-1]['kind'],name='step')
 data=dict(source_is_original_execution=True,scope='declared controlled dispatch; pool/world/graphics restored after each call',source=json.loads((out/'source.json').read_text()),scenario=spec,cases=cases,records=rows)
 dest=ROOT/('docs/research/nes-contra/evidence/enemies-base-'+('waves-controlled'if mode=='waves'else'controlled')+'.json.gz');packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
 if dest.exists():
  old=json.loads(gzip.decompress(dest.read_bytes()));assert old['cases']==cases and old['records']==rows,'repeat differs'
 else:dest.write_bytes(packed)
 print(json.dumps(dict(cases=len(rows),sha256=hashlib.sha256(dest.read_bytes()).hexdigest())))
if __name__=='__main__':main()
